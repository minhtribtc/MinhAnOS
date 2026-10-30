package platform

import (
	"errors"
	"net/http"
	"strconv"

	identityschema "minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/storagecore"
)

// storageHandler serves the Storage Manager API (/api/storage/...). Every call needs storage.manage
// through the identity policy (System Admin); writes also need a reauthentication within five minutes.
// storagecore writes the storage audit in the same transaction as each change.
func (a *App) storageHandler() http.Handler {
	mux := http.NewServeMux()
	write := func(handler func(w http.ResponseWriter, r *http.Request, actor string)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, _ := r.Cookie("minhanos_session")
			if cookie == nil || a.storageDenied(w, r, a.identityService.Fresh(r.Context(), identityschema.DesktopRealm, cookie.Value)) {
				return
			}
			actor, err := identityschema.Repository{DB: a.db}.PublicUser(r.Context(), identity(r).User.ID)
			if err != nil {
				internalError(w, err)
				return
			}
			handler(w, r, actor.ID)
		}
	}
	mux.HandleFunc("GET /api/storage/volumes", func(w http.ResponseWriter, r *http.Request) {
		volumes, err := a.storage.Volumes(r.Context())
		if err != nil && volumes == nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": volumes, "agentError": err != nil})
	})
	mux.HandleFunc("GET /api/storage/shares", func(w http.ResponseWriter, r *http.Request) {
		shares, err := a.storage.Shares(r.Context())
		if err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": shares})
	})
	mux.HandleFunc("POST /api/storage/shares", write(func(w http.ResponseWriter, r *http.Request, actor string) {
		var body struct {
			Key        string `json:"key"`
			Name       string `json:"name"`
			Volume     string `json:"volume"`
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !decode(w, r, &body) {
			return
		}
		share, err := a.storage.CreateShare(r.Context(), actor, body.Key, body.Name, body.Volume, body.LimitBytes)
		if err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 201, share)
	}))
	mux.HandleFunc("PUT /api/storage/shares/{id}/quota", write(func(w http.ResponseWriter, r *http.Request, actor string) {
		var body struct {
			Version    int64  `json:"version"`
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := a.storage.SetShareLimit(r.Context(), actor, r.PathValue("id"), body.Version, body.LimitBytes); err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("GET /api/storage/homes", func(w http.ResponseWriter, r *http.Request) {
		homes, err := a.storage.Homes(r.Context())
		if err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": homes})
	})
	mux.HandleFunc("PUT /api/storage/homes/{user}/quota", write(func(w http.ResponseWriter, r *http.Request, actor string) {
		var body struct {
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := a.storage.SetHomeLimit(r.Context(), actor, r.PathValue("user"), body.LimitBytes); err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("POST /api/storage/homes/{user}/measure", write(func(w http.ResponseWriter, r *http.Request, actor string) {
		if err := a.storage.RequestMeasure(r.Context(), actor, r.PathValue("user")); err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 202, map[string]bool{"ok": true})
	}))
	mux.HandleFunc("GET /api/storage/settings", func(w http.ResponseWriter, r *http.Request) {
		limit, err := a.storage.DefaultHomeLimit(r.Context())
		if err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"defaultHomeLimitBytes": limit})
	})
	mux.HandleFunc("PUT /api/storage/settings", write(func(w http.ResponseWriter, r *http.Request, actor string) {
		var body struct {
			DefaultHomeLimitBytes *int64 `json:"defaultHomeLimitBytes"`
		}
		if !decode(w, r, &body) {
			return
		}
		if err := a.storage.SetDefaultHomeLimit(r.Context(), actor, body.DefaultHomeLimitBytes); err != nil {
			a.storageFailure(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.identityService == nil || a.storageDenied(w, r, a.identityService.CheckApp(r.Context(), identity(r).User.ID, identityschema.StorageApp, "storage.manage")) {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// storageDenied writes the identity policy error (if any) and reports whether the request must stop.
func (a *App) storageDenied(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var f *identityschema.Fault
	switch {
	case errors.As(err, &f):
		writeError(w, f.Status, "Không có quyền quản lý lưu trữ.", f.Code)
	case errors.Is(err, identityschema.ErrUnauthorized):
		writeError(w, 401, "Phiên đăng nhập đã hết hạn.", "AUTH_REQUIRED")
	default:
		internalError(w, err)
	}
	return true
}

func (a *App) storageFailure(w http.ResponseWriter, err error) {
	var agent storagecore.AgentError
	switch {
	case errors.Is(err, storagecore.ErrInvalid):
		writeError(w, 400, "Thông tin lưu trữ không hợp lệ.", "STORAGE_INVALID")
	case errors.Is(err, storagecore.ErrNotFound):
		writeError(w, 404, "Không tìm thấy.", "STORAGE_NOT_FOUND")
	case errors.Is(err, storagecore.ErrExists):
		writeError(w, 409, "Đã tồn tại.", "STORAGE_EXISTS")
	case errors.Is(err, storagecore.ErrConflict):
		writeError(w, 412, "Dữ liệu đã thay đổi; hãy làm mới.", "VERSION_CONFLICT")
	case errors.Is(err, storagecore.ErrNoAgent):
		writeError(w, 503, "Storage agent chưa được cấu hình.", "STORAGE_AGENT_UNAVAILABLE")
	case errors.As(err, &agent):
		writeError(w, 502, "Storage agent từ chối thao tác ("+agent.Code+", "+strconv.Itoa(agent.Status)+").", "STORAGE_AGENT_ERROR")
	default:
		internalError(w, err)
	}
}
