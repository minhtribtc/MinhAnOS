package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	identityschema "minhanos.local/platform/internal/identity"
	"minhanos.local/platform/internal/requestid"
	"minhanos.local/platform/internal/storagecore"
)

// storageHandler serves the Storage Manager API v1 (/api/v1/storage/..., contracts/storage). System Admins
// (storage.manage) manage everything. Organization admins manage the shares, quota folders and permissions
// of their organizations, and managers manage the allocations of descendant organizations (ADR 0008). Each
// resource is authorized through its owning organization; one the caller may not manage reads as not
// found. Writes need reauthentication within five minutes, PUT/DELETE need If-Match and POST needs
// Idempotency-Key. Errors use the nested v1 envelope.
func (a *App) storageHandler() http.Handler {
	mux := http.NewServeMux()
	type writeFunc func(w http.ResponseWriter, r *http.Request, actor string, raw []byte)
	write := func(handler writeFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			cookie, _ := r.Cookie("minhanos_session")
			if cookie == nil {
				v1Error(w, r, 401, "AUTH_REQUIRED")
				return
			}
			if a.v1Denied(w, r, a.identityService.Fresh(r.Context(), identityschema.DesktopRealm, cookie.Value)) {
				return
			}
			actor, err := identityschema.Repository{DB: a.db}.PublicUser(r.Context(), identity(r).User.ID)
			if err != nil {
				internalError(w, r, err)
				return
			}
			raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32768))
			if err != nil {
				v1Error(w, r, 413, "PAYLOAD_TOO_LARGE")
				return
			}
			if r.Method != "POST" {
				handler(w, r, actor.ID, raw)
				return
			}
			key := r.Header.Get("Idempotency-Key")
			if len(key) < 16 || len(key) > 128 {
				v1Error(w, r, 400, "IDEMPOTENCY_KEY_REQUIRED")
				return
			}
			keySum, reqSum := sha256.Sum256([]byte(key)), sha256.Sum256([]byte(r.URL.Path+"\x00"+string(raw)))
			keyHash, reqHash := hex.EncodeToString(keySum[:]), hex.EncodeToString(reqSum[:])
			status, body, found, err := a.storage.Replay(r.Context(), actor.ID, keyHash, reqHash)
			switch {
			case errors.Is(err, storagecore.ErrIdempotencyConflict):
				v1Error(w, r, 409, "IDEMPOTENCY_CONFLICT")
				return
			case err != nil:
				internalError(w, r, err)
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
					logError(r.Context(), "storage.idempotency_not_stored", err)
				}
			}
		}
	}
	decodeStrict := func(w http.ResponseWriter, r *http.Request, raw []byte, v any) bool {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return false
		}
		return true
	}
	ifMatch := func(w http.ResponseWriter, r *http.Request) (int64, bool) {
		v := r.Header.Get("If-Match")
		if v == "" {
			v1Error(w, r, 428, "PRECONDITION_REQUIRED")
			return 0, false
		}
		n, err := strconv.ParseInt(strings.Trim(v, `"`), 10, 64)
		if err != nil || n < 1 {
			v1Error(w, r, 412, "VERSION_CONFLICT")
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
		if sc := scopeOf(r); !sc.root {
			// Organization admins see only the pools their organizations have an allocation on.
			allocations, e := a.storage.Allocations(r.Context(), sc.ids())
			if e != nil {
				a.storageFailure(w, r, e)
				return
			}
			allowed := map[string]bool{}
			for _, x := range allocations {
				allowed[x.Volume] = true
			}
			kept := volumes[:0]
			for _, v := range volumes {
				if allowed[v.Name] {
					kept = append(kept, v)
				}
			}
			volumes = kept
		}
		writeJSON(w, 200, map[string]any{"items": volumes, "agentError": err != nil})
	})
	mux.HandleFunc("GET /api/v1/storage/shares", func(w http.ResponseWriter, r *http.Request) {
		shares, err := a.storage.Shares(r.Context())
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		if sc := scopeOf(r); !sc.root {
			kept := shares[:0]
			for _, s := range shares {
				if s.OrganizationID != nil && sc.admin(*s.OrganizationID) {
					kept = append(kept, s)
				}
			}
			shares = kept
		}
		writeJSON(w, 200, map[string]any{"items": shares})
	})
	mux.HandleFunc("POST /api/v1/storage/shares", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		var body struct {
			Key        string `json:"key"`
			Name       string `json:"name"`
			Volume     string `json:"volume"`
			LimitBytes *int64 `json:"limitBytes"`
			HostsHomes bool   `json:"hostsHomes"`
			// Owner; omitted means the default organization (System Admin only).
			OrganizationID *string `json:"organizationId"`
		}
		if !decodeStrict(w, r, raw, &body) {
			return
		}
		org := ""
		if body.OrganizationID != nil {
			org = *body.OrganizationID
		}
		if sc := scopeOf(r); !sc.root && (body.HostsHomes || !sc.admin(org)) {
			v1Error(w, r, 403, "STORAGE_PERMISSION_DENIED")
			return
		}
		share, err := a.storage.CreateOrgShare(r.Context(), actor, org, body.Key, body.Name, body.Volume, body.LimitBytes, body.HostsHomes)
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
	mux.HandleFunc("GET /api/v1/storage/shares/{id}/access", func(w http.ResponseWriter, r *http.Request) {
		list, err := a.storage.ShareAccess(r.Context(), r.PathValue("id"))
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%d"`, list.Version))
		writeJSON(w, 200, list)
	})
	mux.HandleFunc("PUT /api/v1/storage/shares/{id}/access", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		var body struct {
			Entries []storagecore.AccessEntry `json:"entries"`
		}
		if !ok || !decodeStrict(w, r, raw, &body) {
			return
		}
		if body.Entries == nil {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return
		}
		if err := a.storage.SetShareAccess(r.Context(), actor, r.PathValue("id"), version, body.Entries); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("POST /api/v1/storage/shares/{id}/measure", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		if len(bytes.TrimSpace(raw)) > 0 && string(bytes.TrimSpace(raw)) != "{}" {
			v1Error(w, r, 400, "INVALID_REQUEST")
			return
		}
		if err := a.storage.RequestShareMeasure(r.Context(), actor, r.PathValue("id")); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 202)
	}))
	mux.HandleFunc("GET /api/v1/storage/shares/{id}/folders", func(w http.ResponseWriter, r *http.Request) {
		items, err := a.storage.QuotaFolders(r.Context(), r.PathValue("id"))
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/storage/shares/{id}/folders", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		var body struct {
			Path       string `json:"path"`
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !decodeStrict(w, r, raw, &body) {
			return
		}
		folder, err := a.storage.CreateQuotaFolder(r.Context(), actor, r.PathValue("id"), body.Path, body.LimitBytes)
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%d"`, folder.Version))
		writeJSON(w, 201, folder)
	}))
	mux.HandleFunc("PUT /api/v1/storage/folders/{id}/quota", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		var body struct {
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !ok || !decodeStrict(w, r, raw, &body) {
			return
		}
		if err := a.storage.SetQuotaFolderLimit(r.Context(), actor, r.PathValue("id"), version, body.LimitBytes); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("DELETE /api/v1/storage/folders/{id}", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		if !ok {
			return
		}
		if err := a.storage.DeleteQuotaFolder(r.Context(), actor, r.PathValue("id"), version); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("GET /api/v1/storage/subjects", func(w http.ResponseWriter, r *http.Request) {
		org := r.URL.Query().Get("organization")
		if sc := scopeOf(r); !sc.root && !sc.admin(org) {
			v1Error(w, r, 404, "STORAGE_NOT_FOUND")
			return
		}
		items, err := a.storage.Subjects(r.Context(), org)
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
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
			v1Error(w, r, 400, "INVALID_REQUEST")
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
	mux.HandleFunc("GET /api/v1/storage/organizations", func(w http.ResponseWriter, r *http.Request) {
		sc := scopeOf(r)
		ids := sc.ids()
		if sc.root {
			ids = nil
		}
		allocations, err := a.storage.Allocations(r.Context(), ids)
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		byOrg := map[string][]storagecore.Allocation{}
		for _, x := range allocations {
			byOrg[x.OrganizationID] = append(byOrg[x.OrganizationID], x)
		}
		items := []map[string]any{}
		for _, o := range sc.list {
			list := byOrg[o.ID]
			if list == nil {
				list = []storagecore.Allocation{}
			}
			items = append(items, map[string]any{"id": o.ID, "name": o.Name, "parentId": o.ParentID, "status": o.Status, "role": o.Role, "allocations": list})
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/v1/storage/organizations/{org}/allocations", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		var body struct {
			Volume     string `json:"volume"`
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !decodeStrict(w, r, raw, &body) {
			return
		}
		org := r.PathValue("org")
		if !scopeOf(r).allocates(org) {
			v1Error(w, r, 404, "STORAGE_NOT_FOUND")
			return
		}
		allocation, err := a.storage.CreateAllocation(r.Context(), actor, org, body.Volume, body.LimitBytes)
		if err != nil {
			a.storageFailure(w, r, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%d"`, allocation.Version))
		writeJSON(w, 201, allocation)
	}))
	allocationOwner := func(w http.ResponseWriter, r *http.Request) bool {
		org, err := a.storage.AllocationOrganization(r.Context(), r.PathValue("id"))
		if err == nil && !scopeOf(r).allocates(org) {
			err = storagecore.ErrNotFound
		}
		if err != nil {
			a.storageFailure(w, r, err)
			return false
		}
		return true
	}
	mux.HandleFunc("PUT /api/v1/storage/allocations/{id}", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		var body struct {
			LimitBytes *int64 `json:"limitBytes"`
		}
		if !ok || !decodeStrict(w, r, raw, &body) || !allocationOwner(w, r) {
			return
		}
		if err := a.storage.SetAllocationLimit(r.Context(), actor, r.PathValue("id"), version, body.LimitBytes); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("DELETE /api/v1/storage/allocations/{id}", write(func(w http.ResponseWriter, r *http.Request, actor string, raw []byte) {
		version, ok := ifMatch(w, r)
		if !ok || !allocationOwner(w, r) {
			return
		}
		if err := a.storage.DeleteAllocation(r.Context(), actor, r.PathValue("id"), version); err != nil {
			a.storageFailure(w, r, err)
			return
		}
		accepted(w, 200)
	}))
	mux.HandleFunc("/api/v1/storage/", func(w http.ResponseWriter, r *http.Request) { v1Error(w, r, 404, "STORAGE_NOT_FOUND") })
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if a.identityService == nil {
			v1Error(w, r, 503, "SERVICE_UNAVAILABLE")
			return
		}
		sc, err := a.storageScope(r)
		if a.v1Denied(w, r, err) {
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/storage/"), "/")
		if !sc.root {
			// Homes and platform settings are System Admin only; a share or quota folder is managed by the
			// direct admins of its organization and otherwise reads as not found (no ID probing).
			switch parts[0] {
			case "volumes", "shares", "folders", "subjects", "organizations", "allocations":
			default:
				v1Error(w, r, 403, "STORAGE_PERMISSION_DENIED")
				return
			}
			if len(parts) >= 2 && (parts[0] == "shares" || parts[0] == "folders") {
				share := parts[1]
				if parts[0] == "folders" {
					if share, err = a.storage.FolderShare(r.Context(), parts[1]); err != nil {
						a.storageFailure(w, r, err)
						return
					}
				}
				org, kind, err := a.storage.ShareOwner(r.Context(), share)
				if err == nil && (kind != "share" || !sc.admin(org)) {
					err = storagecore.ErrNotFound
				}
				if err != nil {
					a.storageFailure(w, r, err)
					return
				}
			}
		}
		mux.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), storageScopeKey{}, sc)))
	})
}

// storageScope is what the caller may manage in Storage Manager (ADR 0008): everything (System Admin), or the
// organizations they administer (data) and manage (descendants' allocations).
type storageScope struct {
	root bool
	list []identityschema.Organization // organizations the caller has an admin, manager or root role in
	role map[string]string
}
type storageScopeKey struct{}

func scopeOf(r *http.Request) storageScope {
	return r.Context().Value(storageScopeKey{}).(storageScope)
}

func (s storageScope) admin(org string) bool { return s.root || (org != "" && s.role[org] == "admin") }

// allocates: top-level allocations are Root's; a descendant's are its managers' (never its own admins').
func (s storageScope) allocates(org string) bool {
	if s.root {
		_, ok := s.role[org]
		return ok
	}
	return s.role[org] == "manager"
}
func (s storageScope) ids() []string {
	out := []string{}
	for _, o := range s.list {
		out = append(out, o.ID)
	}
	return out
}

// storageScope: System Admin via storage.manage; otherwise organization roles. An explicit deny rule on the
// Storage app is never bypassed by an organization role.
func (a *App) storageScope(r *http.Request) (storageScope, error) {
	user := identity(r).User.ID
	sc := storageScope{role: map[string]string{}}
	err := a.identityService.CheckApp(r.Context(), user, identityschema.StorageApp, "storage.manage")
	var f *identityschema.Fault
	if err != nil && !(errors.As(err, &f) && f.Code == "IDENTITY_PERMISSION_DENIED") {
		return sc, err
	}
	sc.root = err == nil
	orgs, e := a.identityService.Organizations(r.Context(), user)
	if e != nil {
		return sc, e
	}
	for _, o := range orgs {
		if sc.root || o.Role == "admin" || o.Role == "manager" {
			sc.list = append(sc.list, o)
			sc.role[o.ID] = o.Role
		}
	}
	if !sc.root && len(sc.list) == 0 {
		return sc, err
	}
	return sc, nil
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

// v1Error writes the nested v1 envelope (storage, notifications, logs); the code is stable, the message is
// not shown to users.
func v1Error(w http.ResponseWriter, r *http.Request, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": code, "requestId": requestid.From(r.Context()).Request, "details": map[string]any{}}})
}

// v1Denied writes the identity policy error (if any) and reports whether the request must stop.
func (a *App) v1Denied(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	var f *identityschema.Fault
	switch {
	case errors.As(err, &f):
		v1Error(w, r, f.Status, f.Code)
	case errors.Is(err, identityschema.ErrUnauthorized):
		v1Error(w, r, 401, "AUTH_REQUIRED")
	default:
		internalError(w, r, err)
	}
	return true
}

func (a *App) storageFailure(w http.ResponseWriter, r *http.Request, err error) {
	var agent storagecore.AgentError
	switch {
	case errors.Is(err, storagecore.ErrInvalid):
		v1Error(w, r, 400, "STORAGE_INVALID")
	case errors.Is(err, storagecore.ErrNotFound):
		v1Error(w, r, 404, "STORAGE_NOT_FOUND")
	case errors.Is(err, storagecore.ErrExists):
		v1Error(w, r, 409, "STORAGE_EXISTS")
	case errors.Is(err, storagecore.ErrHomesExists):
		v1Error(w, r, 409, "STORAGE_HOMES_EXISTS")
	case errors.Is(err, storagecore.ErrNoHomes):
		v1Error(w, r, 409, "STORAGE_NOT_READY")
	case errors.Is(err, storagecore.ErrConflict):
		v1Error(w, r, 412, "VERSION_CONFLICT")
	case errors.Is(err, storagecore.ErrNoAllocation):
		v1Error(w, r, 409, "STORAGE_NO_ALLOCATION")
	case errors.Is(err, storagecore.ErrNoParentAllocation):
		v1Error(w, r, 409, "STORAGE_PARENT_ALLOCATION_REQUIRED")
	case errors.Is(err, storagecore.ErrAllocationExceeded):
		v1Error(w, r, 409, "STORAGE_ALLOCATION_EXCEEDED")
	case errors.Is(err, storagecore.ErrAllocationCommitted):
		v1Error(w, r, 409, "STORAGE_ALLOCATION_COMMITTED")
	case errors.Is(err, storagecore.ErrNotEmpty):
		v1Error(w, r, 409, "STORAGE_NOT_EMPTY")
	case errors.Is(err, storagecore.ErrNoAgent):
		v1Error(w, r, 503, "STORAGE_AGENT_UNAVAILABLE")
	case errors.As(err, &agent):
		v1Error(w, r, 502, "STORAGE_AGENT_ERROR")
	default:
		internalError(w, r, err)
	}
}
