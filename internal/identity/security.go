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
	if e = s.Auth.attempt(ctx, session.Principal.Username); e != nil {
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
		if _, e := tx.Exec(ctx, "UPDATE users SET password_hash=$2,auth_epoch=auth_epoch+1 WHERE id=$1", a.StorageID, hash); e != nil {
			return Result{}, e
		}
		if e := revoke(ctx, tx, a.StorageID, "password.changed"); e != nil {
			return Result{}, e
		}
		return result(map[string]bool{"accepted": true}, 200, 0), nil
	})
}
