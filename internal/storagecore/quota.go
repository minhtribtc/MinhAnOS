// Package storagecore owns the storage registry (volumes, shared folders) and the application-level
// soft quota ledger shared by Files and the Storage Manager (docs/storage-model-review.md, Q1).
package storagecore

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
	"minhanos.local/platform/internal/requestid"
)

//go:embed migrations/001_registry_quota.sql
var registrySQL string

//go:embed migrations/002_contract.sql
var contractSQL string

//go:embed migrations/003_share_access.sql
var shareAccessSQL string

//go:embed migrations/004_quota_folders.sql
var quotaFoldersSQL string

//go:embed migrations/005_quota_notices.sql
var quotaNoticesSQL string

//go:embed migrations/006_allocations.sql
var allocationsSQL string

func Module() migrate.Module {
	return migrate.Module{ID: "storage", Steps: []migrate.Step{{ID: 1, Name: "registry_quota", SQL: registrySQL}, {ID: 2, Name: "contract", SQL: contractSQL}, {ID: 3, Name: "share_access", SQL: shareAccessSQL}, {ID: 4, Name: "quota_folders", SQL: quotaFoldersSQL}, {ID: 5, Name: "quota_notices", SQL: quotaNoticesSQL}, {ID: 6, Name: "allocations", SQL: allocationsSQL}}}
}

func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}

// reservationTTL bounds how long an interrupted write can hold space before it is released.
const reservationTTL = 15 * time.Minute

// QuotaExceeded names the scope that refused a reservation.
type QuotaExceeded struct{ Scope string }

func (e QuotaExceeded) Error() string { return "quota exceeded: " + e.Scope }

type Store struct{ DB *pgxpool.Pool }

// Usage is one scope's ledger. Logical bytes written through MinhAnOS; not physical or snapshot space.
type Usage struct {
	LimitBytes    *int64 `json:"limitBytes"`
	UsedBytes     int64  `json:"usedBytes"`
	ReservedBytes int64  `json:"reservedBytes"`
	Measured      bool   `json:"measured"`
}

type Reservation struct {
	ID, Home, Share string
	Folders         []string // quota folders the write lands in (Q3)
	Bytes           int64
}

func (s Store) homesShare(ctx context.Context, tx pgx.Tx) (string, error) {
	var id string
	e := tx.QueryRow(ctx, "SELECT id::text FROM storage_shares WHERE kind='homes'").Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", ErrNoHomes
	}
	return id, e
}

// ensureHome creates a home ledger row with the default quota the first time a user writes.
func ensureHome(ctx context.Context, tx pgx.Tx, home string) error {
	_, e := tx.Exec(ctx, `INSERT INTO storage_quota(scope_type,scope_id,limit_bytes) VALUES('home',$1,(SELECT (value#>>'{}')::bigint FROM storage_settings WHERE key='default_home_quota_bytes')) ON CONFLICT DO NOTHING`, home)
	return e
}

// releaseExpired returns space held by writes that never finished (bounded batch).
func releaseExpired(ctx context.Context, tx pgx.Tx) error {
	_, e := tx.Exec(ctx, `WITH gone AS (DELETE FROM storage_reservations WHERE id IN (SELECT id FROM storage_reservations WHERE expires_at<now() LIMIT 100) RETURNING home_id,share_id,folder_ids,bytes),
 f AS (UPDATE storage_quota q SET reserved_bytes=greatest(0,q.reserved_bytes-x.bytes) FROM (SELECT fid,sum(bytes) bytes FROM gone,unnest(folder_ids) fid GROUP BY 1) x WHERE q.scope_type='folder' AND q.scope_id=x.fid),
 h AS (UPDATE storage_quota q SET reserved_bytes=greatest(0,q.reserved_bytes-x.bytes) FROM (SELECT home_id,sum(bytes) bytes FROM gone GROUP BY 1) x WHERE q.scope_type='home' AND q.scope_id=x.home_id)
 UPDATE storage_quota q SET reserved_bytes=greatest(0,q.reserved_bytes-x.bytes) FROM (SELECT share_id,sum(bytes) bytes FROM gone GROUP BY 1) x WHERE q.scope_type='share' AND q.scope_id=x.share_id`)
	return e
}

// Reserve holds bytes on the user's home and on the share hosting homes before a write. Both scopes must
// fit (used + reserved + bytes <= limit) or nothing is reserved.
func (s Store) Reserve(ctx context.Context, home string, bytes int64) (Reservation, error) {
	return s.reserve(ctx, home, "", nil, bytes)
}

// ReserveShare holds bytes on a Shared Folder (its total quota) and on the quota folders the write lands in.
// An empty share reserves on the folders only (a move inside the share does not change its total).
func (s Store) ReserveShare(ctx context.Context, share string, folders []string, bytes int64) (Reservation, error) {
	return s.reserve(ctx, "", share, folders, bytes)
}

type scope struct{ kind, id string }

// scopes are the ledger rows a reservation touches: the home, its share, quota folders (each when set).
func (r Reservation) scopes() []scope {
	result := []scope{}
	if r.Home != "" {
		result = append(result, scope{"home", r.Home})
	}
	if r.Share != "" {
		result = append(result, scope{"share", r.Share})
	}
	for _, f := range r.Folders {
		result = append(result, scope{"folder", f})
	}
	return result
}

func (s Store) reserve(ctx context.Context, home, share string, folders []string, bytes int64) (Reservation, error) {
	if bytes < 0 {
		return Reservation{}, fmt.Errorf("negative reservation")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return Reservation{}, e
	}
	defer tx.Rollback(ctx)
	if e = releaseExpired(ctx, tx); e != nil {
		return Reservation{}, e
	}
	if home != "" {
		if e = ensureHome(ctx, tx, home); e != nil {
			return Reservation{}, e
		}
		if share, e = s.homesShare(ctx, tx); e != nil {
			return Reservation{}, e
		}
	}
	for _, scope := range (Reservation{Home: home, Share: share, Folders: folders}).scopes() {
		tag, e := tx.Exec(ctx, "UPDATE storage_quota SET reserved_bytes=reserved_bytes+$3 WHERE scope_type=$1 AND scope_id=$2 AND (limit_bytes IS NULL OR used_bytes+reserved_bytes+$3<=limit_bytes)", scope.kind, scope.id, bytes)
		if allocationExceeded(e) {
			return Reservation{}, QuotaExceeded{"allocation"}
		}
		if e != nil {
			return Reservation{}, e
		}
		if tag.RowsAffected() != 1 {
			return Reservation{}, QuotaExceeded{scope.kind}
		}
	}
	r := Reservation{Home: home, Share: share, Folders: folders, Bytes: bytes}
	if folders == nil {
		folders = []string{}
	}
	if e = tx.QueryRow(ctx, "INSERT INTO storage_reservations(home_id,share_id,folder_ids,bytes,expires_at) VALUES($1,$2,$3,$4,now()+make_interval(secs=>$5)) RETURNING id::text", home, share, folders, bytes, reservationTTL.Seconds()).Scan(&r.ID); e != nil {
		return Reservation{}, e
	}
	return r, tx.Commit(ctx)
}

// tagRequest lets the quota notice trigger (SQL 005) record the request and correlation IDs of the change.
func tagRequest(ctx context.Context, tx pgx.Tx) error {
	ids := requestid.From(ctx)
	_, e := tx.Exec(ctx, "SELECT set_config('minhanos.request_id', $1, true), set_config('minhanos.correlation_id', $2, true)", ids.Request, ids.Correlation)
	return e
}

// settle ends a reservation; written bytes (actual) become usage. If the reservation already expired and
// was released, only the usage is added, so nothing is subtracted twice.
func (s Store) settle(ctx context.Context, r Reservation, actual int64) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = tagRequest(ctx, tx); e != nil {
		return e
	}
	var held int64
	e = tx.QueryRow(ctx, "DELETE FROM storage_reservations WHERE id=$1 RETURNING bytes", r.ID).Scan(&held)
	if errors.Is(e, pgx.ErrNoRows) {
		held = 0 // expired and already released by releaseExpired
	} else if e != nil {
		return e
	}
	for _, scope := range r.scopes() {
		if _, e = tx.Exec(ctx, "UPDATE storage_quota SET used_bytes=greatest(0,used_bytes+$3),reserved_bytes=greatest(0,reserved_bytes-$4) WHERE scope_type=$1 AND scope_id=$2", scope.kind, scope.id, actual, held); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// Commit records the bytes a finished write added (may differ from the reservation).
func (s Store) Commit(ctx context.Context, r Reservation, actual int64) error {
	return s.settle(ctx, r, actual)
}

// Release drops a reservation after a failed write.
func (s Store) Release(ctx context.Context, r Reservation) error { return s.settle(ctx, r, 0) }

// NeedsMeasure reports whether the home's usage must be (re)measured before it is trusted.
func (s Store) NeedsMeasure(ctx context.Context, home string) (bool, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return false, e
	}
	defer tx.Rollback(ctx)
	if e = ensureHome(ctx, tx, home); e != nil {
		return false, e
	}
	var need bool
	if e = tx.QueryRow(ctx, "SELECT measure_requested FROM storage_quota WHERE scope_type='home' AND scope_id=$1", home).Scan(&need); e != nil {
		return false, e
	}
	return need, tx.Commit(ctx)
}

// SetMeasured replaces the home's usage with a filesystem measurement taken while Files holds the user's
// write lock, and moves the homes share total by the same difference.
func (s Store) SetMeasured(ctx context.Context, home string, bytes int64) error {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	if e = ensureHome(ctx, tx, home); e != nil {
		return e
	}
	var before int64
	if e = tx.QueryRow(ctx, "SELECT used_bytes FROM storage_quota WHERE scope_type='home' AND scope_id=$1 FOR UPDATE", home).Scan(&before); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_quota SET used_bytes=$2,measure_requested=false,measured_at=now() WHERE scope_type='home' AND scope_id=$1", home, bytes); e != nil {
		return e
	}
	share, e := s.homesShare(ctx, tx)
	if e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_quota SET used_bytes=greatest(0,used_bytes+$2) WHERE scope_type='share' AND scope_id=$1", share, bytes-before); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// HomeUsage returns the ledger of one home (creating it with the default quota when missing).
func (s Store) HomeUsage(ctx context.Context, home string) (Usage, error) {
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return Usage{}, e
	}
	defer tx.Rollback(ctx)
	if e = ensureHome(ctx, tx, home); e != nil {
		return Usage{}, e
	}
	var u Usage
	if e = tx.QueryRow(ctx, "SELECT limit_bytes,used_bytes,reserved_bytes,NOT measure_requested FROM storage_quota WHERE scope_type='home' AND scope_id=$1", home).Scan(&u.LimitBytes, &u.UsedBytes, &u.ReservedBytes, &u.Measured); e != nil {
		return Usage{}, e
	}
	return u, tx.Commit(ctx)
}

// PendingMeasures lists homes waiting for a background measurement (bounded batch).
func (s Store) PendingMeasures(ctx context.Context, limit int) ([]string, error) {
	rows, e := s.DB.Query(ctx, "SELECT scope_id FROM storage_quota WHERE scope_type='home' AND measure_requested ORDER BY scope_id LIMIT $1", limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		result = append(result, id)
	}
	return result, rows.Err()
}
