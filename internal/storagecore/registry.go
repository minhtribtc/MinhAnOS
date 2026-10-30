package storagecore

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"minhanos.local/platform/internal/requestid"
)

var (
	ErrNotFound = errors.New("storage object not found")
	ErrInvalid  = errors.New("invalid storage request")
	ErrExists   = errors.New("storage object already exists")
	ErrConflict = errors.New("storage object changed")
	shareKey    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
)

// maxLimit keeps quota values far from int64 overflow.
const maxLimit = int64(1) << 60

type Volume struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Managed   bool   `json:"managed"` // created through the storage agent (ZFS/Btrfs)
	Quota     bool   `json:"hardQuota"`
	Snapshots bool   `json:"snapshots"`
	Online    bool   `json:"online"`
}

type Share struct {
	ID            string `json:"id"`
	Key           string `json:"key"`
	Name          string `json:"name"`
	Volume        string `json:"volume"`
	Kind          string `json:"kind"`
	Version       int64  `json:"version"`
	Usage         Usage  `json:"usage"`
	PhysicalBytes *int64 `json:"physicalBytes"` // filesystem-reported, ZFS/Btrfs only
}

type Home struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Share       string `json:"share"`
	Version     int64  `json:"version"` // quota policy version for If-Match
	Usage       Usage  `json:"usage"`
}

// Manager is the Storage Manager backend. Callers authorize (identity policy) before every call.
type Manager struct {
	Store Store
	Agent AgentClient
}

func event(ctx context.Context, tx pgx.Tx, actor, action, target string, details map[string]any) error {
	ids := requestid.From(ctx)
	if details == nil {
		details = map[string]any{}
	}
	_, e := tx.Exec(ctx, "INSERT INTO storage_events(actor_id,action,target,request_id,correlation_id,details) VALUES(nullif($1,'')::uuid,$2,$3,$4,$5,$6)", actor, action, target, ids.Request, ids.Correlation, details)
	return e
}

// Volumes merges the agent's volumes into the registry; volumes the agent no longer reports stay listed offline.
func (m Manager) Volumes(ctx context.Context) ([]Volume, error) {
	agent, agentErr := m.Agent.Volumes(ctx)
	online := map[string]AgentVolume{}
	for _, v := range agent {
		online[v.Name] = v
		if _, e := m.Store.DB.Exec(ctx, "INSERT INTO storage_volumes(name,kind) VALUES($1,$2) ON CONFLICT(name) DO NOTHING", v.Name, v.Kind); e != nil {
			return nil, e
		}
	}
	rows, e := m.Store.DB.Query(ctx, "SELECT name,kind FROM storage_volumes ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Volume{}
	for rows.Next() {
		var v Volume
		if e = rows.Scan(&v.Name, &v.Kind); e != nil {
			return nil, e
		}
		a, ok := online[v.Name]
		v.Managed = v.Kind != "directory"
		v.Online = v.Kind == "directory" || ok
		v.Quota, v.Snapshots = a.Capabilities.Quota, a.Capabilities.Snapshots
		result = append(result, v)
	}
	if agentErr != nil && !errors.Is(agentErr, ErrNoAgent) {
		return result, agentErr
	}
	return result, rows.Err()
}

func (m Manager) Shares(ctx context.Context) ([]Share, error) {
	rows, e := m.Store.DB.Query(ctx, `SELECT s.id::text,s.key,s.name,s.volume,s.kind,s.version,q.limit_bytes,q.used_bytes,q.reserved_bytes,NOT q.measure_requested FROM storage_shares s JOIN storage_quota q ON q.scope_type='share' AND q.scope_id=s.id::text ORDER BY s.kind DESC,s.name`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Share{}
	for rows.Next() {
		var s Share
		if e = rows.Scan(&s.ID, &s.Key, &s.Name, &s.Volume, &s.Kind, &s.Version, &s.Usage.LimitBytes, &s.Usage.UsedBytes, &s.Usage.ReservedBytes, &s.Usage.Measured); e != nil {
			return nil, e
		}
		result = append(result, s)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	rows.Close()
	for i := range result {
		if result[i].Kind == "homes" {
			result[i].Usage.Measured = true // total of the homes ledgers
			continue
		}
		if unit, e := m.Agent.Share(ctx, result[i].Volume, result[i].Key); e == nil {
			n := int64(unit.ReferencedBytes)
			result[i].PhysicalBytes = &n
		}
	}
	return result, nil
}

func validLimit(limit *int64) bool { return limit == nil || (*limit >= 0 && *limit <= maxLimit) }

// CreateShare reserves the key in a transaction, creates the ZFS dataset / Btrfs subvolume through the
// agent, then commits; an agent failure rolls the registry back. A share that hosts homes needs no unit of
// its own: each home becomes a unit homes/<user> on the same volume when first used. The creator of an
// ordinary share gets write access so it is usable at once (audited; editable in Storage Manager).
func (m Manager) CreateShare(ctx context.Context, actor, key, name, volume string, limit *int64, hostsHomes bool) (Share, error) {
	name = strings.TrimSpace(name)
	if !shareKey.MatchString(key) || (key == "homes" && !hostsHomes) || len([]rune(name)) < 1 || len([]rune(name)) > 100 || !validLimit(limit) {
		return Share{}, ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return Share{}, e
	}
	defer tx.Rollback(ctx)
	var kind string
	if e = tx.QueryRow(ctx, "SELECT kind FROM storage_volumes WHERE name=$1", volume).Scan(&kind); errors.Is(e, pgx.ErrNoRows) {
		return Share{}, ErrNotFound
	} else if e != nil {
		return Share{}, e
	}
	if kind == "directory" {
		return Share{}, ErrInvalid // plain volumes only host the legacy homes area
	}
	s := Share{Key: key, Name: name, Volume: volume, Kind: "share", Version: 1}
	if hostsHomes {
		s.Kind = "homes"
		var exists bool
		if e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_shares WHERE kind='homes')").Scan(&exists); e != nil {
			return Share{}, e
		}
		if exists {
			return Share{}, ErrHomesExists
		}
	}
	e = tx.QueryRow(ctx, "INSERT INTO storage_shares(key,name,volume,kind) VALUES($1,$2,$3,$4) ON CONFLICT(key) DO NOTHING RETURNING id::text", key, name, volume, s.Kind).Scan(&s.ID)
	if errors.Is(e, pgx.ErrNoRows) {
		return Share{}, ErrExists
	}
	if e != nil {
		return Share{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO storage_quota(scope_type,scope_id,limit_bytes,measure_requested,measured_at) VALUES('share',$1,$2,false,now())", s.ID, limit); e != nil {
		return Share{}, e
	}
	if e = event(ctx, tx, actor, "storage.share.created", s.ID, map[string]any{"key": key, "volume": volume, "hostsHomes": hostsHomes}); e != nil {
		return Share{}, e
	}
	if hostsHomes {
		s.Usage = Usage{LimitBytes: limit, Measured: true}
		return s, tx.Commit(ctx)
	}
	if uuidRE.MatchString(actor) {
		if _, e = tx.Exec(ctx, "INSERT INTO storage_share_access(share_id,subject_type,subject_id,access) VALUES($1,'user',$2,'write')", s.ID, actor); e != nil {
			return Share{}, e
		}
	}
	if _, e = m.Agent.CreateShare(ctx, volume, key); e != nil {
		var ae AgentError
		if errors.As(e, &ae) && ae.Code == "STORAGE_EXISTS" {
			return Share{}, ErrExists
		}
		return Share{}, e
	}
	s.Usage = Usage{LimitBytes: limit, Measured: true}
	return s, tx.Commit(ctx)
}

func (m Manager) SetShareLimit(ctx context.Context, actor, id string, version int64, limit *int64) error {
	if !validLimit(limit) {
		return ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE storage_shares SET version=version+1 WHERE id::text=$1 AND version=$2", id, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		var exists bool
		tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_shares WHERE id::text=$1)", id).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
		return ErrConflict
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_quota SET limit_bytes=$2,version=version+1 WHERE scope_type='share' AND scope_id=$1", id, limit); e != nil {
		return e
	}
	if e = event(ctx, tx, actor, "storage.share.quota", id, map[string]any{"limitBytes": limit}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// Homes lists every non-deleted account with its home ledger (users without a row show unmeasured).
func (m Manager) Homes(ctx context.Context) ([]Home, error) {
	share := ""
	if h, e := m.Store.HomesShare(ctx); e == nil {
		share = h.Key
	} else if !errors.Is(e, ErrNoHomes) {
		return nil, e
	}
	rows, e := m.Store.DB.Query(ctx, `SELECT u.public_id::text,u.username,u.display_name,coalesce(q.version,1),q.limit_bytes,coalesce(q.used_bytes,0),coalesce(q.reserved_bytes,0),coalesce(NOT q.measure_requested,false) FROM users u LEFT JOIN storage_quota q ON q.scope_type='home' AND q.scope_id=u.id WHERE u.status<>'deleted' AND u.public_id IS NOT NULL ORDER BY u.username`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Home{}
	for rows.Next() {
		h := Home{Share: share}
		if e = rows.Scan(&h.UserID, &h.Username, &h.DisplayName, &h.Version, &h.Usage.LimitBytes, &h.Usage.UsedBytes, &h.Usage.ReservedBytes, &h.Usage.Measured); e != nil {
			return nil, e
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

func storageKey(ctx context.Context, tx pgx.Tx, publicID string) (string, error) {
	var id string
	e := tx.QueryRow(ctx, "SELECT id FROM users WHERE public_id::text=$1 AND status<>'deleted'", publicID).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, e
}

// SetHomeLimit sets one user's quota; usage above a new lower limit is kept (no data is removed) but
// further writes are refused until the user is below the limit again.
func (m Manager) SetHomeLimit(ctx context.Context, actor, userID string, version int64, limit *int64) error {
	if !validLimit(limit) {
		return ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	home, e := storageKey(ctx, tx, userID)
	if e != nil {
		return e
	}
	if e = ensureHome(ctx, tx, home); e != nil {
		return e
	}
	tag, e := tx.Exec(ctx, "UPDATE storage_quota SET limit_bytes=$2,version=version+1 WHERE scope_type='home' AND scope_id=$1 AND version=$3", home, limit, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if e = event(ctx, tx, actor, "storage.home.quota", userID, map[string]any{"limitBytes": limit}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// RequestMeasure asks Files to re-measure a home on its next access (repairs drift, never enforces).
func (m Manager) RequestMeasure(ctx context.Context, actor, userID string) error {
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	home, e := storageKey(ctx, tx, userID)
	if e != nil {
		return e
	}
	if e = ensureHome(ctx, tx, home); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_quota SET measure_requested=true WHERE scope_type='home' AND scope_id=$1", home); e != nil {
		return e
	}
	if e = event(ctx, tx, actor, "storage.home.measure", userID, nil); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// DefaultHomeLimit returns the default quota for new homes and the setting's version (If-Match).
func (m Manager) DefaultHomeLimit(ctx context.Context) (*int64, int64, error) {
	var v *int64
	var version int64
	e := m.Store.DB.QueryRow(ctx, "SELECT (value#>>'{}')::bigint,version FROM storage_settings WHERE key='default_home_quota_bytes'").Scan(&v, &version)
	return v, version, e
}

// SetDefaultHomeLimit applies to homes created afterwards; existing homes keep their limit.
func (m Manager) SetDefaultHomeLimit(ctx context.Context, actor string, version int64, limit *int64) error {
	if !validLimit(limit) {
		return ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE storage_settings SET value=coalesce(to_jsonb($1::bigint),'null'::jsonb),version=version+1 WHERE key='default_home_quota_bytes' AND version=$2", limit, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if e = event(ctx, tx, actor, "storage.default.quota", "default_home_quota_bytes", map[string]any{"limitBytes": limit}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// Replay returns the stored response of an earlier POST with the same Idempotency-Key. A different request
// under the same key is ErrIdempotencyConflict. Keys live 24 hours; expired ones are removed in small batches.
func (m Manager) Replay(ctx context.Context, actor, keyHash, requestHash string) (int, []byte, bool, error) {
	if _, e := m.Store.DB.Exec(ctx, "DELETE FROM storage_idempotency WHERE (actor_id,key_hash) IN (SELECT actor_id,key_hash FROM storage_idempotency WHERE expires_at<now() LIMIT 100)"); e != nil {
		return 0, nil, false, e
	}
	var previous string
	var status int
	var body []byte
	e := m.Store.DB.QueryRow(ctx, "SELECT request_hash,status,response FROM storage_idempotency WHERE actor_id=$1 AND key_hash=$2", actor, keyHash).Scan(&previous, &status, &body)
	if errors.Is(e, pgx.ErrNoRows) {
		return 0, nil, false, nil
	}
	if e != nil {
		return 0, nil, false, e
	}
	if previous != requestHash {
		return 0, nil, false, ErrIdempotencyConflict
	}
	return status, body, true, nil
}

// Remember stores a successful POST response for replay.
func (m Manager) Remember(ctx context.Context, actor, keyHash, requestHash string, status int, body []byte) error {
	_, e := m.Store.DB.Exec(ctx, "INSERT INTO storage_idempotency(actor_id,key_hash,request_hash,status,response,expires_at) VALUES($1,$2,$3,$4,$5,now()+interval '24 hours') ON CONFLICT DO NOTHING", actor, keyHash, requestHash, status, body)
	return e
}

var ErrIdempotencyConflict = errors.New("idempotency key reused for a different request")
