package identity

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"minhanos.local/platform/internal/authn"
)

func (s *Service) RevokeSession(ctx context.Context, realm Realm, token, handle, key, requestID string, others bool, raw []byte) (Result, error) {
	route := "DELETE /api/v1/identity/me/sessions/" + handle
	if others {
		route = "POST /api/v1/identity/me/sessions/revoke-others"
	}
	return s.mutate(ctx, realm, token, "identity.sessions.revoke", requestID, key, route, raw, false, func(tx pgx.Tx, a Actor) (Result, error) {
		if others {
			current, _ := table(realm)
			for _, t := range []string{"sessions", "drive_sessions"} {
				if _, e := tx.Exec(ctx, "DELETE FROM "+t+" WHERE user_id=$1 AND NOT($2 AND token_hash=$3)", a.StorageID, t == current, authn.Digest(token)); e != nil {
					return Result{}, e
				}
			}
		} else {
			p := strings.Split(handle, ":")
			if len(p) != 2 || !uuidPattern.MatchString(p[1]) {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			t, e := table(Realm(p[0]))
			if e != nil {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			tag, e := tx.Exec(ctx, "DELETE FROM "+t+" WHERE user_id=$1 AND public_id::text=$2", a.StorageID, p[1])
			if e != nil {
				return Result{}, e
			}
			if tag.RowsAffected() != 1 {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
		}
		return result(map[string]bool{"accepted": true}, 200, 0), nil
	})
}

type PasswordChange struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Service) Password(ctx context.Context, realm Realm, token, key, requestID string, input PasswordChange, raw []byte) (Result, error) {
	if len([]rune(input.NewPassword)) < 12 || len(input.NewPassword) > 256 {
		return Result{}, ErrPasswordPolicy
	}
	if len(input.CurrentPassword) < 1 || len(input.CurrentPassword) > 256 {
		return Result{}, fault(401, "INVALID_CREDENTIALS")
	}
	if e := s.preflight(ctx, realm, token, "identity.password.change", false); e != nil {
		return Result{}, e
	}
	release, e := s.Auth.acquire()
	if e != nil {
		return Result{}, e
	}
	defer release()
	session, e := s.Auth.Authenticate(ctx, realm, token)
	if e != nil {
		return Result{}, e
	}
	if _, e = s.Auth.attempt(ctx, session.Principal.Username); e != nil {
		return Result{}, e
	}
	var expected string
	if e = s.db.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", session.Principal.StorageID).Scan(&expected); e != nil {
		return Result{}, noRows(e)
	}
	if !authn.Verify(input.CurrentPassword, expected) {
		return Result{}, fault(401, "INVALID_CREDENTIALS")
	}
	hash, e := authn.Hash(input.NewPassword)
	if e != nil {
		return Result{}, e
	}
	return s.mutate(ctx, realm, token, "identity.password.change", requestID, key, "POST /api/v1/identity/me/password", raw, false, func(tx pgx.Tx, a Actor) (Result, error) {
		var freshHash string
		if e := tx.QueryRow(ctx, "SELECT password_hash FROM users WHERE id=$1", a.StorageID).Scan(&freshHash); e != nil {
			return Result{}, e
		}
		if freshHash != expected {
			return Result{}, ErrUnauthorized
		}
		if _, e := tx.Exec(ctx, "UPDATE users SET password_hash=$2,auth_epoch=auth_epoch+1,password_change_required=false WHERE id=$1", a.StorageID, hash); e != nil {
			return Result{}, e
		}
		if e := revoke(ctx, tx, a.StorageID); e != nil {
			return Result{}, e
		}
		if e := notify(ctx, tx, a.StorageID, "password.changed", "warning", true, nil); e != nil {
			return Result{}, e
		}
		return result(map[string]bool{"accepted": true}, 200, 0), nil
	})
}

type PasswordReset struct {
	NewPassword string `json:"newPassword"`
	// RequireChange (default true) makes the user choose their own password at next sign-in.
	RequireChange *bool `json:"requireChange"`
}

// ResetPassword lets an administrator set another user's password without knowing the old one.
// It needs identity.users.write, recent reauthentication and the same target ceiling as other
// user changes; the target's sessions in both realms are revoked. Own password uses Password.
func (s *Service) ResetPassword(ctx context.Context, realm Realm, token, target, match, key, requestID string, input PasswordReset, raw []byte) (Result, error) {
	if len([]rune(input.NewPassword)) < 12 || len(input.NewPassword) > 256 {
		return Result{}, ErrPasswordPolicy
	}
	if e := s.preflight(ctx, realm, token, "identity.users.write", true); e != nil {
		return Result{}, e
	}
	// Hash before taking the policy lock; the KDF is bounded by AuthCore's worker limit.
	hash, e := s.Auth.PasswordHash(input.NewPassword)
	if e != nil {
		return Result{}, e
	}
	route := "POST /api/v1/identity/users/" + target + "/password"
	return s.mutate(ctx, realm, token, "identity.users.write", requestID, key, route, append([]byte(match+"\x00"), raw...), true, func(tx pgx.Tx, a Actor) (Result, error) {
		if target == a.PublicID {
			return Result{}, fault(409, "IDENTITY_USE_OWN_PASSWORD")
		}
		u, role, e := userRow(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if e = targetAllowed(ctx, tx, a, u, role); e != nil {
			return Result{}, e
		}
		if e = requireVersion(u.Version, match); e != nil {
			return Result{}, e
		}
		if u.Status == "deleted" {
			return Result{}, fault(409, "IDENTITY_USER_DELETED")
		}
		var storageID string
		var operator bool
		if e = tx.QueryRow(ctx, "SELECT u.id,EXISTS(SELECT 1 FROM identity_host_operators o WHERE o.user_id=u.id AND o.state='enabled') FROM users u WHERE u.public_id=$1", target).Scan(&storageID, &operator); e != nil {
			return Result{}, e
		}
		// Taking over a host operator's password would grant host access through the web;
		// operators change their own password or use the OS-root recovery CLI.
		if operator {
			return Result{}, fault(403, "IDENTITY_HOST_OPERATOR_PROTECTED")
		}
		if _, e = tx.Exec(ctx, "UPDATE users SET password_hash=$2,auth_epoch=auth_epoch+1,password_change_required=$3 WHERE id=$1", storageID, hash, input.RequireChange == nil || *input.RequireChange); e != nil {
			return Result{}, e
		}
		if e = revoke(ctx, tx, storageID); e != nil {
			return Result{}, e
		}
		if e = notify(ctx, tx, storageID, "password.reset", "critical", true, nil); e != nil {
			return Result{}, e
		}
		if u, _, e = userRow(ctx, tx, target); e != nil {
			return Result{}, e
		}
		return result(u, 200, u.Version), nil
	})
}

// RevokeUserSessions signs another user out of both realms (e.g. a lost device). It needs
// identity.users.sessions.revoke, recent reauthentication and the usual target ceiling.
func (s *Service) RevokeUserSessions(ctx context.Context, realm Realm, token, target, key, requestID string, raw []byte) (Result, error) {
	route := "POST /api/v1/identity/users/" + target + "/sessions/revoke"
	return s.mutate(ctx, realm, token, "identity.users.sessions.revoke", requestID, key, route, raw, true, func(tx pgx.Tx, a Actor) (Result, error) {
		if target == a.PublicID {
			return Result{}, fault(409, "IDENTITY_USE_OWN_SESSIONS")
		}
		u, role, e := userRow(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if e = targetAllowed(ctx, tx, a, u, role); e != nil {
			return Result{}, e
		}
		storageID, e := storageIDOf(ctx, tx, target)
		if e != nil {
			return Result{}, e
		}
		if e = revoke(ctx, tx, storageID); e != nil {
			return Result{}, e
		}
		if e = notify(ctx, tx, storageID, "sessions.revoked", "warning", true, nil); e != nil {
			return Result{}, e
		}
		return result(map[string]bool{"accepted": true}, 200, 0), nil
	})
}

// passwordChangeAllowed lists what a session may do while an administrative reset requires
// the user to choose a new password: read themselves and change the password.
func passwordChangeAllowed(method, path string) bool {
	switch method + " " + path {
	case "GET me", "GET me/access", "GET me/credentials", "POST reauthenticate", "POST me/password":
		return true
	}
	return false
}
