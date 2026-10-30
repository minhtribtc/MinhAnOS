package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	identityschema "minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/logcore"
	"minhanos.local/platform/internal/notifycore"
)

// notificationsHandler serves Notification Center API v1 (/api/v1/notifications/..., contracts/notifications):
// the signed-in user's own inbox. Reading needs notifications.own.read, marking read notifications.own.manage
// (every account, identity SQL 011). Marking read is idempotent by itself, so the Idempotency-Key is checked
// but no replay is stored.
func (a *App) notificationsHandler() http.Handler {
	mux := http.NewServeMux()
	recipient := func(w http.ResponseWriter, r *http.Request, permission string) (string, bool) {
		if a.v1Denied(w, r, a.identityService.CheckApp(r.Context(), identity(r).User.ID, identityschema.NotificationsApp, permission)) {
			return "", false
		}
		user, err := identityschema.Repository{DB: a.db}.PublicUser(r.Context(), identity(r).User.ID)
		if err != nil {
			internalError(w, err)
			return "", false
		}
		return user.ID, true
	}
	failure := func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, notifycore.ErrInvalid) {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return
		}
		internalError(w, err)
	}
	mux.HandleFunc("GET /api/v1/notifications/inbox", func(w http.ResponseWriter, r *http.Request) {
		id, ok := recipient(w, r, "notifications.own.read")
		if !ok {
			return
		}
		q := r.URL.Query()
		limit, unread := 20, q.Get("unread")
		if v := q.Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				v1Error(w, r, 400, "INVALID_REQUEST")
				return
			}
			limit = n
		}
		if unread != "" && unread != "true" && unread != "false" {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return
		}
		page, err := a.notifications.List(r.Context(), id, unread == "true", q.Get("cursor"), limit)
		if err != nil {
			failure(w, r, err)
			return
		}
		writeJSON(w, 200, page)
	})
	mux.HandleFunc("POST /api/v1/notifications/inbox/read", func(w http.ResponseWriter, r *http.Request) {
		if key := r.Header.Get("Idempotency-Key"); len(key) < 16 || len(key) > 128 {
			v1Error(w, r, 400, "IDEMPOTENCY_KEY_REQUIRED")
			return
		}
		id, ok := recipient(w, r, "notifications.own.manage")
		if !ok {
			return
		}
		var body struct {
			IDs  []string   `json:"ids"`
			UpTo *time.Time `json:"upTo"`
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			v1Error(w, r, 413, "PAYLOAD_TOO_LARGE")
			return
		}
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(&body) != nil || d.Decode(&struct{}{}) != io.EOF {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return
		}
		updated, unread, err := a.notifications.MarkRead(r.Context(), id, body.IDs, body.UpTo)
		if err != nil {
			failure(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]int64{"updated": updated, "unreadCount": unread})
	})
	mux.HandleFunc("/api/v1/notifications/", func(w http.ResponseWriter, r *http.Request) { v1Error(w, r, 404, "NOTIFICATION_NOT_FOUND") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if a.identityService == nil {
			v1Error(w, r, 503, "SERVICE_UNAVAILABLE")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// logsHandler serves Log Center API v1 (/api/v1/logs/..., contracts/logs): read-only audit events of every
// module. Needs logs.audit.read (System Admin, identity SQL 011).
func (a *App) logsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/logs/sources", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"items": logcore.Sources})
	})
	mux.HandleFunc("GET /api/v1/logs/events", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := logcore.Filter{Source: q.Get("source"), Outcome: q.Get("outcome"), Cursor: q.Get("cursor"), Limit: 50}
		if v := q.Get("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				v1Error(w, r, 400, "INVALID_REQUEST")
				return
			}
			f.Limit = n
		}
		page, err := a.logs.Events(r.Context(), f)
		if errors.Is(err, logcore.ErrInvalid) {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return
		}
		if err != nil {
			internalError(w, err)
			return
		}
		writeJSON(w, 200, page)
	})
	mux.HandleFunc("/api/v1/logs/", func(w http.ResponseWriter, r *http.Request) { v1Error(w, r, 404, "LOG_NOT_FOUND") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if a.identityService == nil {
			v1Error(w, r, 503, "SERVICE_UNAVAILABLE")
			return
		}
		if a.v1Denied(w, r, a.identityService.CheckApp(r.Context(), identity(r).User.ID, identityschema.LogsApp, "logs.audit.read")) {
			return
		}
		mux.ServeHTTP(w, r)
	})
}
