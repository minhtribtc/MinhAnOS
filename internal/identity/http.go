package identity

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"minhanos.local/platform/internal/requestid"
)

// HTTPHandler serves the implemented v1 slice in either host; it never imports
// a Shell, trusts identity headers or enables the still-planned registry routes.
func (s *Service) HTTPHandler(realm Realm) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hosts wrap requests in requestid.Middleware; a bare handler (tests) gets fresh IDs.
		requestID := requestid.From(r.Context()).Request
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		cookieName := "minhanos_session"
		if realm == DriveRealm {
			cookieName = "minhanos_drive_session"
		}
		token := ""
		if c, e := r.Cookie(cookieName); e == nil {
			token = c.Value
		}
		session, e := s.Auth.Authenticate(ctx, realm, token)
		fail := func(err error) {
			status, code := 500, "INTERNAL_ERROR"
			var f *Fault
			switch {
			case errors.As(err, &f):
				status, code = f.Status, f.Code
			case errors.Is(err, ErrUnauthorized):
				status, code = 401, "AUTH_REQUIRED"
			case errors.Is(err, ErrThrottled):
				status, code = 429, "AUTH_RATE_LIMITED"
			case errors.Is(err, ErrBusy):
				status, code = 503, "AUTH_BUSY"
			case errors.Is(err, ErrNotReady):
				status, code = 503, "IDENTITY_NOT_READY"
			case errors.Is(err, ErrPasswordPolicy):
				status, code = 422, "IDENTITY_PASSWORD_POLICY"
			}
			if status == 403 {
				s.RecordDenied(ctx, session.Principal.StorageID, requestID, r.Method+" "+r.URL.Path, code)
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": code, "message": code, "requestId": requestID, "details": map[string]any{}}})
		}
		if e != nil {
			fail(e)
			return
		}
		if e = s.Ready(ctx); e != nil {
			fail(e)
			return
		}
		write := r.Method != "GET" && r.Method != "HEAD"
		if write {
			origin := r.Header.Get("Origin")
			ok := r.Header.Get("Sec-Fetch-Site") != "cross-site"
			if origin != "" {
				u, e := url.Parse(origin)
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				ok = ok && e == nil && u.Scheme == scheme && u.User == nil && u.Path == "" && u.RawQuery == "" && u.Fragment == "" && strings.EqualFold(u.Host, r.Host)
			}
			if !ok {
				fail(fault(403, "IDENTITY_ORIGIN_DENIED"))
				return
			}
			if session.CSRF == "" || subtle.ConstantTimeCompare([]byte(session.CSRF), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
				fail(fault(403, "IDENTITY_CSRF_DENIED"))
				return
			}
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/identity/")
		parts := strings.Split(path, "/")
		if session.Principal.PasswordChangeRequired && !passwordChangeAllowed(r.Method, path) {
			fail(fault(403, "PASSWORD_CHANGE_REQUIRED"))
			return
		}
		var out Result
		if r.Method == "GET" {
			kind, target := path, ""
			if len(parts) == 2 && parts[0] == "users" {
				kind, target = "user", parts[1]
			}
			if len(parts) == 3 && parts[0] == "users" && parts[2] == "access" {
				kind, target = "user/access", parts[1]
			}
			if len(parts) == 3 && parts[0] == "groups" && parts[2] == "members" {
				kind, target = "members", parts[1]
			}
			if len(parts) == 2 && parts[0] == "groups" {
				kind, target = "group", parts[1]
			}
			if len(parts) == 3 && parts[0] == "users" && parts[2] == "sessions" {
				kind, target = "user/sessions", parts[1]
			}
			if len(parts) == 3 && parts[0] == "applications" && parts[2] == "access" {
				kind, target = "app/access", parts[1]
			}
			limit := 50
			if v := r.URL.Query().Get("limit"); v != "" {
				limit, e = strconv.Atoi(v)
				if e != nil {
					fail(fault(400, "INVALID_REQUEST"))
					return
				}
			}
			// member filters groups by one user; target filters audit events by subject (actor or target).
			search := r.URL.Query().Get("search")
			for key := range r.URL.Query() {
				switch {
				case key == "limit" || key == "search" || key == "cursor":
				case key == "member" && kind == "groups", key == "target" && kind == "audit-events":
					if search != "" {
						fail(fault(400, "INVALID_REQUEST"))
						return
					}
					search = r.URL.Query().Get(key)
				default:
					fail(fault(400, "INVALID_REQUEST"))
					return
				}
			}
			out, e = s.Read(ctx, realm, token, kind, target, search, r.URL.Query().Get("cursor"), limit)
		} else {
			raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
			if err != nil {
				fail(fault(413, "PAYLOAD_TOO_LARGE"))
				return
			}
			if len(raw) > 0 && strings.TrimSpace(strings.ToLower(strings.Split(r.Header.Get("Content-Type"), ";")[0])) != "application/json" {
				fail(fault(415, "UNSUPPORTED_MEDIA_TYPE"))
				return
			}
			decode := func(v any) error {
				d := json.NewDecoder(strings.NewReader(string(raw)))
				d.DisallowUnknownFields()
				if e := d.Decode(v); e != nil {
					return fault(400, "INVALID_REQUEST")
				}
				if d.Decode(&struct{}{}) != io.EOF {
					return fault(400, "INVALID_REQUEST")
				}
				return nil
			}
			key := r.Header.Get("Idempotency-Key")
			match := r.Header.Get("If-Match")
			switch {
			case path == "reauthenticate" && r.Method == "POST":
				var b struct {
					Password string `json:"password"`
				}
				if e = decode(&b); e == nil {
					e = s.Auth.Reauthenticate(ctx, realm, token, b.Password)
					if errors.Is(e, ErrUnauthorized) {
						e = fault(401, "INVALID_CREDENTIALS")
					}
					out = result(map[string]bool{"accepted": true}, 200, 0)
				}
			case path == "me/password" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input PasswordChange
				if e = decode(&input); e == nil {
					out, e = s.Password(ctx, realm, token, key, requestID, input, raw)
				}
			case path == "me/sessions/revoke-others" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.RevokeSession(ctx, realm, token, "", key, requestID, true, raw)
			case len(parts) == 3 && parts[0] == "me" && parts[1] == "sessions" && r.Method == "DELETE":
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.RevokeSession(ctx, realm, token, parts[2], "", requestID, false, raw)
			case path == "groups" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input GroupInput
				if e = decode(&input); e == nil {
					out, e = s.GroupWrite(ctx, realm, token, "", "create", "", match, key, requestID, input, raw)
				}
			case len(parts) == 2 && parts[0] == "groups" && r.Method == "PATCH":
				var input struct {
					Name *string `json:"name"`
				}
				if e = decode(&input); e != nil {
					break
				}
				if input.Name == nil {
					e = fault(400, "INVALID_REQUEST")
					break
				}
				out, e = s.GroupWrite(ctx, realm, token, parts[1], "patch", "", match, "", requestID, GroupInput{Name: *input.Name}, raw)
			case len(parts) == 2 && parts[0] == "groups" && r.Method == "DELETE":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.GroupWrite(ctx, realm, token, parts[1], "delete", "", match, key, requestID, GroupInput{}, raw)
			case len(parts) == 3 && parts[0] == "groups" && parts[2] == "members" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input struct {
					UserID string `json:"userId"`
				}
				if e = decode(&input); e == nil {
					out, e = s.GroupWrite(ctx, realm, token, parts[1], "add", input.UserID, match, key, requestID, GroupInput{}, raw)
				}
			case len(parts) == 4 && parts[0] == "groups" && parts[2] == "members" && r.Method == "DELETE":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.GroupWrite(ctx, realm, token, parts[1], "remove", parts[3], match, key, requestID, GroupInput{}, raw)
			case path == "roles" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input RoleInput
				if e = decode(&input); e == nil {
					out, e = s.CreateRole(ctx, realm, token, key, requestID, input, raw)
				}
			case path == "role-bindings" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input BindingInput
				if e = decode(&input); e == nil {
					var shape struct {
						Scope map[string]json.RawMessage `json:"scope"`
					}
					_ = json.Unmarshal(raw, &shape)
					if _, ok := shape.Scope["id"]; !ok {
						e = fault(400, "INVALID_REQUEST")
						break
					}
					out, e = s.BindRole(ctx, realm, token, "", match, key, requestID, input, raw, false)
				}
			case len(parts) == 2 && parts[0] == "role-bindings" && r.Method == "DELETE":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.BindRole(ctx, realm, token, parts[1], match, key, requestID, BindingInput{}, raw, true)
			case path == "users" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var b CreateUser
				if e = decode(&b); e == nil {
					out, e = s.Create(ctx, realm, token, b, "user", key, requestID, raw, false)
				}
			case len(parts) == 2 && parts[0] == "users" && r.Method == "PATCH" || path == "me" && r.Method == "PATCH":
				var b struct {
					DisplayName *string `json:"displayName"`
				}
				if e = decode(&b); e != nil {
					break
				}
				if b.DisplayName == nil {
					e = fault(400, "INVALID_REQUEST")
					break
				}
				target := parts[len(parts)-1]
				operation := "patch"
				if path == "me" {
					u, err := (Repository{s.db}).PublicUser(ctx, session.Principal.StorageID)
					if err != nil {
						e = err
						break
					}
					target = u.ID
					operation = "self"
				}
				out, e = s.ChangeUser(ctx, realm, token, target, operation, *b.DisplayName, match, "", requestID, raw)
			case len(parts) == 3 && parts[0] == "users" && (parts[2] == "disable" || parts[2] == "enable") && r.Method == "POST" || len(parts) == 2 && parts[0] == "users" && r.Method == "DELETE":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(strings.TrimSpace(string(raw))) > 0 && string(raw) != "{}" {
					e = fault(400, "INVALID_REQUEST")
					break
				}
				operation := "delete"
				if len(parts) == 3 {
					operation = parts[2]
				}
				out, e = s.ChangeUser(ctx, realm, token, parts[1], operation, "", match, key, requestID, raw)
			case len(parts) == 3 && parts[0] == "applications" && parts[2] == "access" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input AppAccessInput
				if e = decode(&input); e == nil {
					out, e = s.AppAccess(ctx, realm, token, parts[1], "", key, requestID, input, raw, false)
				}
			case len(parts) == 4 && parts[0] == "applications" && parts[2] == "access" && r.Method == "DELETE":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.AppAccess(ctx, realm, token, parts[1], parts[3], key, requestID, AppAccessInput{}, raw, true)
			case len(parts) == 4 && parts[0] == "users" && parts[2] == "sessions" && parts[3] == "revoke" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				if len(raw) > 0 {
					if e = decode(&struct{}{}); e != nil {
						break
					}
				}
				out, e = s.RevokeUserSessions(ctx, realm, token, parts[1], key, requestID, raw)
			case len(parts) == 3 && parts[0] == "users" && parts[2] == "password" && r.Method == "POST":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var input PasswordReset
				if e = decode(&input); e == nil {
					out, e = s.ResetPassword(ctx, realm, token, parts[1], match, key, requestID, input, raw)
				}
			case len(parts) == 3 && parts[0] == "users" && parts[2] == "role" && r.Method == "PUT":
				if key == "" {
					e = fault(400, "IDEMPOTENCY_KEY_REQUIRED")
					break
				}
				var b struct {
					Role string `json:"role"`
				}
				if e = decode(&b); e != nil {
					break
				}
				if b.Role != "system-admin" && b.Role != "user-admin" && b.Role != "standard-user" {
					e = fault(422, "IDENTITY_INVALID_ROLE")
					break
				}
				out, e = s.ChangeUser(ctx, realm, token, parts[1], b.Role, "", match, key, requestID, raw)
			default:
				e = fault(404, "IDENTITY_NOT_FOUND")
			}
		}
		if e != nil {
			fail(e)
			return
		}
		if out.ETag != "" {
			w.Header().Set("ETag", out.ETag)
		}
		w.WriteHeader(out.Status)
		w.Write(out.Body)
	})
}
