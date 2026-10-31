package storagecore

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Allocations (ADR 0008): an organization's budget on one pool, carved out of its parent's budget.
// Callers authorize first (Root for top-level organizations, a manager of the organization otherwise).

var (
	// ErrNoAllocation: the organization has no allocation on the pool (a share needs one).
	ErrNoAllocation = errors.New("organization has no allocation on this volume")
	// ErrNoParentAllocation: a child allocation needs one of the parent organization on the same pool.
	ErrNoParentAllocation = errors.New("parent organization has no allocation on this volume")
	// ErrAllocationExceeded: the parent's direct budget cannot cover the request.
	ErrAllocationExceeded = errors.New("allocation exceeds the parent budget")
	// ErrAllocationCommitted: a limit below what is delegated, used and reserved.
	ErrAllocationCommitted = errors.New("allocation below committed bytes")
	// ErrNotEmpty: shares or child allocations still use the allocation.
	ErrNotEmpty = errors.New("allocation still in use")
)

type Allocation struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Volume         string  `json:"volume"`
	ParentID       *string `json:"parentId"`
	LimitBytes     *int64  `json:"limitBytes"`
	DelegatedBytes int64   `json:"delegatedBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	ReservedBytes  int64   `json:"reservedBytes"`
	Version        int64   `json:"version"`
}

const allocationColumns = `a.id::text,a.organization_id::text,a.volume,a.parent_id::text,a.limit_bytes,a.delegated_bytes,coalesce(q.used_bytes,0),coalesce(q.reserved_bytes,0),a.version`

func scanAllocation(row pgx.Row, a *Allocation) error {
	return row.Scan(&a.ID, &a.OrganizationID, &a.Volume, &a.ParentID, &a.LimitBytes, &a.DelegatedBytes, &a.UsedBytes, &a.ReservedBytes, &a.Version)
}

// Allocations lists the allocations of the given organizations (nil: all), bounded.
func (m Manager) Allocations(ctx context.Context, orgs []string) ([]Allocation, error) {
	rows, e := m.Store.DB.Query(ctx, `SELECT `+allocationColumns+` FROM storage_allocations a LEFT JOIN storage_quota q ON q.scope_type='allocation' AND q.scope_id=a.id::text
		WHERE $1::uuid[] IS NULL OR a.organization_id=ANY($1::uuid[]) ORDER BY a.organization_id,a.volume LIMIT 2000`, orgs)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Allocation{}
	for rows.Next() {
		var a Allocation
		if e = scanAllocation(rows, &a); e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AllocationOrganization returns the organization owning an allocation (for authorization).
func (m Manager) AllocationOrganization(ctx context.Context, id string) (string, error) {
	if !uuidRE.MatchString(id) {
		return "", ErrNotFound
	}
	var org string
	e := m.Store.DB.QueryRow(ctx, "SELECT organization_id::text FROM storage_allocations WHERE id=$1", id).Scan(&org)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return org, e
}

type ledger struct {
	limit          *int64
	delegated      int64
	used, reserved int64
}

// lockAllocation locks an allocation and its ledger row.
func lockAllocation(ctx context.Context, tx pgx.Tx, id string) (ledger, error) {
	var l ledger
	e := tx.QueryRow(ctx, `SELECT a.limit_bytes,a.delegated_bytes,q.used_bytes,q.reserved_bytes FROM storage_allocations a JOIN storage_quota q ON q.scope_type='allocation' AND q.scope_id=a.id::text
		WHERE a.id=$1 FOR UPDATE OF a,q`, id).Scan(&l.limit, &l.delegated, &l.used, &l.reserved)
	if errors.Is(e, pgx.ErrNoRows) {
		return l, ErrNotFound
	}
	return l, e
}

// delegate moves delta bytes of a parent's direct budget to (delta > 0) or from (delta < 0) a child.
func delegate(ctx context.Context, tx pgx.Tx, parent string, delta int64) error {
	l, e := lockAllocation(ctx, tx, parent)
	if e != nil {
		return e
	}
	if delta > 0 && l.limit != nil && *l.limit-l.delegated-delta < l.used+l.reserved {
		return ErrAllocationExceeded
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_allocations SET delegated_bytes=greatest(0,delegated_bytes+$2),version=version+1 WHERE id=$1", parent, delta); e != nil {
		return e
	}
	_, e = tx.Exec(ctx, "UPDATE storage_quota q SET limit_bytes=a.limit_bytes-a.delegated_bytes,version=q.version+1 FROM storage_allocations a WHERE a.id=$1 AND q.scope_type='allocation' AND q.scope_id=a.id::text", parent)
	return e
}

// CreateAllocation gives an organization a budget on a pool. Top-level organizations may be unlimited;
// a child needs a limit that fits its parent's direct budget on the same pool.
func (m Manager) CreateAllocation(ctx context.Context, actor, org, volume string, limit *int64) (Allocation, error) {
	if !uuidRE.MatchString(org) || !validLimit(limit) {
		return Allocation{}, ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return Allocation{}, e
	}
	defer tx.Rollback(ctx)
	var kind string
	if e = tx.QueryRow(ctx, "SELECT kind FROM storage_volumes WHERE name=$1", volume).Scan(&kind); errors.Is(e, pgx.ErrNoRows) {
		return Allocation{}, ErrNotFound
	} else if e != nil {
		return Allocation{}, e
	}
	if kind == "directory" {
		return Allocation{}, ErrInvalid
	}
	var parentOrg *string
	if e = tx.QueryRow(ctx, "SELECT parent_id::text FROM identity_organizations WHERE id=$1 FOR SHARE", org).Scan(&parentOrg); errors.Is(e, pgx.ErrNoRows) {
		return Allocation{}, ErrNotFound
	} else if e != nil {
		return Allocation{}, e
	}
	var parent *string
	if parentOrg != nil {
		if limit == nil {
			return Allocation{}, ErrInvalid
		}
		var id string
		if e = tx.QueryRow(ctx, "SELECT id::text FROM storage_allocations WHERE organization_id=$1 AND volume=$2", *parentOrg, volume).Scan(&id); errors.Is(e, pgx.ErrNoRows) {
			return Allocation{}, ErrNoParentAllocation
		} else if e != nil {
			return Allocation{}, e
		}
		if e = delegate(ctx, tx, id, *limit); e != nil {
			return Allocation{}, e
		}
		parent = &id
	}
	a := Allocation{OrganizationID: org, Volume: volume, ParentID: parent, LimitBytes: limit, Version: 1}
	e = tx.QueryRow(ctx, "INSERT INTO storage_allocations(organization_id,volume,parent_id,limit_bytes) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING RETURNING id::text", org, volume, parent, limit).Scan(&a.ID)
	if errors.Is(e, pgx.ErrNoRows) {
		return Allocation{}, ErrExists
	}
	if e != nil {
		return Allocation{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO storage_quota(scope_type,scope_id,limit_bytes,measure_requested,measured_at) VALUES('allocation',$1,$2,false,now())", a.ID, limit); e != nil {
		return Allocation{}, e
	}
	if e = event(ctx, tx, actor, "storage.allocation.created", a.ID, map[string]any{"volume": volume, "limitBytes": limit}); e != nil {
		return Allocation{}, e
	}
	return a, tx.Commit(ctx)
}

// SetAllocationLimit changes a budget (If-Match on the allocation version). The parent row is locked before
// the child, so concurrent changes under one parent are serialized and can never over-allocate.
func (m Manager) SetAllocationLimit(ctx context.Context, actor, id string, version int64, limit *int64) error {
	if !uuidRE.MatchString(id) {
		return ErrNotFound
	}
	if !validLimit(limit) {
		return ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var parent *string
	if e = tx.QueryRow(ctx, "SELECT parent_id::text FROM storage_allocations WHERE id=$1", id).Scan(&parent); errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if parent != nil {
		if limit == nil {
			return ErrInvalid
		}
		// Lock order: parent first (delegate below locks it again, which is a no-op).
		if _, e = lockAllocation(ctx, tx, *parent); e != nil {
			return e
		}
	}
	self, e := lockAllocation(ctx, tx, id)
	if e != nil {
		return e
	}
	var current int64
	if e = tx.QueryRow(ctx, "SELECT version FROM storage_allocations WHERE id=$1", id).Scan(&current); e != nil {
		return e
	}
	if current != version {
		return ErrConflict
	}
	if limit != nil && *limit < self.delegated+self.used+self.reserved {
		return ErrAllocationCommitted
	}
	if parent != nil {
		if e = delegate(ctx, tx, *parent, *limit-*self.limit); e != nil {
			return e
		}
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_allocations SET limit_bytes=$2,version=version+1 WHERE id=$1", id, limit); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_quota q SET limit_bytes=a.limit_bytes-a.delegated_bytes,version=q.version+1 FROM storage_allocations a WHERE a.id=$1 AND q.scope_type='allocation' AND q.scope_id=a.id::text", id); e != nil {
		return e
	}
	if e = event(ctx, tx, actor, "storage.allocation.limit", id, map[string]any{"limitBytes": limit}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// DeleteAllocation removes an unused allocation and returns its budget to the parent.
func (m Manager) DeleteAllocation(ctx context.Context, actor, id string, version int64) error {
	if !uuidRE.MatchString(id) {
		return ErrNotFound
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var parent *string
	var limit *int64
	var current int64
	if e = tx.QueryRow(ctx, "SELECT parent_id::text,limit_bytes,version FROM storage_allocations WHERE id=$1", id).Scan(&parent, &limit, &current); errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if parent != nil {
		if _, e = lockAllocation(ctx, tx, *parent); e != nil {
			return e
		}
	}
	self, e := lockAllocation(ctx, tx, id)
	if e != nil {
		return e
	}
	if current != version {
		return ErrConflict
	}
	if self.used+self.reserved > 0 {
		return ErrNotEmpty
	}
	// The audit row resolves the organization from the allocation, so it is written first.
	if e = event(ctx, tx, actor, "storage.allocation.removed", id, nil); e != nil {
		return e
	}
	sp, e := tx.Begin(ctx)
	if e != nil {
		return e
	}
	if _, e = sp.Exec(ctx, "DELETE FROM storage_allocations WHERE id=$1", id); e != nil {
		sp.Rollback(ctx)
		var pe *pgconn.PgError
		if errors.As(e, &pe) && (pe.Code == "23503" || pe.Code == "23001") {
			return ErrNotEmpty
		}
		return e
	}
	if e = sp.Commit(ctx); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM storage_quota WHERE scope_type='allocation' AND scope_id=$1", id); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM storage_quota_alerts WHERE scope_type='allocation' AND scope_id=$1", id); e != nil {
		return e
	}
	if parent != nil {
		if e = delegate(ctx, tx, *parent, -*limit); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}

// allocationExceeded maps the ledger trigger's refusal (storage SQL 006) to QuotaExceeded.
func allocationExceeded(e error) bool {
	var pe *pgconn.PgError
	return errors.As(e, &pe) && pe.Message == "STORAGE_ALLOCATION_EXCEEDED"
}
