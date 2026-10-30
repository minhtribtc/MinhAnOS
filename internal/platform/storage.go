package platform

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	identityschema "minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/requestid"
	"minhanos.local/platform/internal/storagecore"
)

// storageHandler serves the Storage Manager API v1 (/api/v1/storage/..., contracts/storage). Every call
// needs storage.manage through the identity policy (System Admin); writes need reauthentication within
// five minutes, PUT needs If-Match and POST needs Idempotency-Key. Errors use the nested v1 envelope.
func (a *App) storageHandler() http.Handler {
	mux := http.NewServeMux()
	type writeFunc func(w http.ResponseWriter, r *http.Request, actor string, raw []byte)
	write := func(handler writeFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, _ := r.Cookie("minhanos_session")
			if cookie == nil {
				storageError(w, r, 401, "AUTH_REQUIRED")
				return
			}
			if a.storageDenied(w, r, a.identityService.Fresh(r.Context(), identityschema.DesktopRealm, cookie.Value)) {
				return
			}
			actor, err := identityschema.Repository{DB: a.db}.PublicUser(r.Context(), identity(r).User.ID)
			if err != nil {
				internalError(w, err)
				return
			}
			raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
			if err != nil {
				storageError(w, r, 413, "PAYLOAD_TOO_LARGE")
				return
			}
			if r.Method != "POST" {
				handler(w, r, actor.ID, raw)
				return
			}
			key := r.Header.Get("Idempotency-Key")
			if len(key) < 16 || len(key) > 128 {
				storageError(w, r, 400, "IDEMPOTENCY_KEY_REQUIRED")
				return
			}
			keySum, reqSum := sha256.Sum256([]byte(key)), sha256.Sum256([]byte(r.URL.Path+"\x00"+string(raw)))
			keyHash, reqHash := hex.EncodeToString(keySum[:]), hex.EncodeToString(reqSum[:])
			status, body, found, err := a.storage.Replay(r.Context(), actor.ID, keyHash, reqHash)
			switch {
			case errors.Is(err, storagecore.ErrIdempotencyConflict):
				storageError(w, r, 409, "IDEMPOTENCY_CONFLICT")
				return
			case err != nil:
				internalError(w, err)
				return
			case found:
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(status)
				w.Write(body)
				return
			}
			rec := &recorder{ResponseWriter: w, status: 200}
			handler(rec, r, actor.ID, raw)
			if rec.status < 300 {
				if err = a.storage.Remember(r.Context(), actor.ID, keyHash, reqHash, rec.status, rec.body.Bytes()); err != nil {
					// The change is done; only replay protection is missing for this key.
					log.Print("storage idempotency not stored: ", err)
				}
			}
		}
	}
	decodeStrict := func(w http.ResponseWriter, r *http.Request, raw []byte, v any) bool {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
			storageError(w, r, 400, "INVALID_REQUEST")
			return false
		}
		return true
	}
	ifMatch := func(w http.ResponseWriter, r *http.Request) (int64, bool) {
		v := r.Header.Get("If-Match")
		if v == "" {
			storageError(w, r, 428, "PRECONDITION_REQUIRED")
			return 0, false
		}
		n, err := strconv.ParseInt(strings.Trim(v, `"`), 10, 64)
		if err != nil || n < 1 {
			storageError(w, r, 412, "VERSION_CONFLICT")
			return 0, false
		}
		return n, true
	}
	accepted := func(w http.ResponseWriter, status int) { writeJSON(w, status, map[string]bool{"accepted": true}) }

	mux.HandleFunc("GET /api/v1/storage/volumes", func(w http.ResponseWriter, r *http.Request) {
		volumes, err := a.storage.Volumes(r.Context())
		if err != nil && volumes == nil {
			a.storageFailure(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": volumes, "agentError": err != nil})
	})
	mux.HandleFunc("GET /api/v1/storage/shares", func(w http.ResponseWriter, r *http.Request) {
		shares, err := a.storage.Shares(r.Context())
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": shares})
	})
	mux.HandleFunc("POST /api/v1/storage/shares", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		var body struct {
			Key        string `json:"key"`
			Name       string `json:"name"`
			Volume     string `json:"volume"`
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !decodeStrict(w, r, raw, &body) {
			return
		}
		share, err := a.storage.CreateShare(r.Context(), actor, body.Key, body.Name, body.Volume, body.LimitBytes)
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%d"`, share.Version))
		writeJSON(w, 201, share)
	}))
	mux.HandleFunc("PUT /api/v1/storage/shares/{id}/quota", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		var body struct {
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !ok || !decodeStrict(w, r, raw, &body) {
			return
		}
		if err := a.storage.SetShareLimit(r.Context(), actor, r.PathValue("id"), version, body.LimitBytes); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("GET /api/v1/storage/homes", func(w http.ResponseWriter, r *http.Request) {
		homes, err := a.storage.Homes(r.Context())
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": homes})
	})
	mux.HandleFunc("PUT /api/v1/storage/homes/{user}/quota", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		var body struct {
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !ok || !decodeStrict(w, r, raw, &body) {
			return
		}
		if err := a.storage.SetHomeLimit(r.Context(), actor, r.PathValue("user"), version, body.LimitBytes); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("POST /api/v1/storage/homes/{user}/measure", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		if len(bytes.TrimSpace(raw)) > 0 && string(bytes.TrimSpace(raw)) != "{}" {
			storageError(w, r, 400, "INVALID_REQUEST")
			return
		}
		if err := a.storage.RequestMeasure(r.Context(), actor, r.PathValue("user")); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 202)
	}))
	mux.HandleFunc("GET /api/v1/storage/settings", func(w http.ResponseWriter, r *http.Request) {
		limit, version, err := a.storage.DefaultHomeLimit(r.Context())
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%d"`, version))
		writeJSON(w, 200, map[string]any{"defaultHomeLimitBytes": limit, "version": version})
	})
	mux.HandleFunc("PUT /api/v1/storage/settings", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		var body struct {
			DefaultHomeLimitBytes *int64 `json:"defaultHomeLimitBytes"`
		}
		if !ok || !decodeStrict(w, r, raw, &body) {
			return
		}
		if err := a.storage.SetDefaultHomeLimit(r.Context(), actor, version, body.DefaultHomeLimitBytes); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("/api/v1/storage/", func(w http.ResponseWriter, r *http.Request) { storageError(w, r, 404, "STORAGE_NOT_FOUND") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if a.identityService == nil || a.storageDenied(w, r, a.identityService.CheckApp(r.Context(), identity(r).User.ID, identityschema.StorageApp, "storage.manage")) {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// recorder keeps a copy of a POST response so it can be replayed for the same Idempotency-Key.
type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (r *recorder) WriteHeader(status int) { r.status = status; r.ResponseWriter.WriteHeader(status) }
func (r *recorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// storageError writes the nested v1 envelope; the code is stable, the message is not shown to users.
func storageError(w http.ResponseWriter, r *http.Request, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": code, "requestId": requestid.From(r.Context()).Request, "details": map[string]any{}}})
}

// storageDenied writes the identity policy error (if any) and reports whether the request must stop.
func (a *App) storageDenied(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var f *identityschema.Fault
	switch {
	case errors.As(err, &f):
		storageError(w, r, f.Status, f.Code)
	case errors.Is(err, identityschema.ErrUnauthorized):
		storageError(w, r, 401, "AUTH_REQUIRED")
	default:
		internalError(w, err)
	}
	return true
}

func (a *App) storageFailure(w http.ResponseWriter, r *http.Request, err error) {
	var agent storagecore.AgentError
	switch {
	case errors.Is(err, storagecore.ErrInvalid):
		storageError(w, r, 400, "STORAGE_INVALID")
	case errors.Is(err, storagecore.ErrNotFound):
		storageError(w, r, 404, "STORAGE_NOT_FOUND")
	case errors.Is(err, storagecore.ErrExists):
		storageError(w, r, 409, "STORAGE_EXISTS")
	case errors.Is(err, storagecore.ErrConflict):
		storageError(w, r, 412, "VERSION_CONFLICT")
	case errors.Is(err, storagecore.ErrNoAgent):
		storageError(w, r, 503, "STORAGE_AGENT_UNAVAILABLE")
	case errors.As(err, &agent):
		storageError(w, r, 502, "STORAGE_AGENT_ERROR")
	default:
		internalError(w, err)
	}
}
