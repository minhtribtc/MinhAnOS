package storagecore

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// MaxQuotaFolders bounds the quota folders of one share (Files loads them on every write).
const MaxQuotaFolders = 100

// ValidFolderPath accepts a clean relative path inside a share ("projects/2026"): no empty, "." or ".."
// segments, no backslash, NUL or newline, each segment at most 255 bytes (Files' name rule).
func ValidFolderPath(p string) bool {
	if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.ContainsAny(p, "\\\x00\n\r") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." || len(s) > 255 {
			return false
		}
	}
	return true
}

// QuotaFolder is a registered folder of a Shared Folder with its own soft quota. Version is the quota
// policy version (If-Match). Path is relative to the share root as Files shows it.
type QuotaFolder struct {
	ID      string `json:"id"`
	ShareID string `json:"shareId"`
	Path    string `json:"path"`
	Version int64  `json:"version"`
	Usage   Usage  `json:"usage"`
}

func (m Manager) QuotaFolders(ctx context.Context, shareID string) ([]QuotaFolder, error) {
	if !uuidRE.MatchString(shareID) {
		return nil, ErrNotFound
	}
	var exists bool
	if e := m.Store.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_shares WHERE id=$1)", shareID).Scan(&exists); e != nil {
		return nil, e
	}
	if !exists {
		return nil, ErrNotFound
	}
	rows, e := m.Store.DB.Query(ctx, `SELECT f.id::text,f.share_id::text,f.path,q.version,q.limit_bytes,q.used_bytes,q.reserved_bytes,NOT q.measure_requested
		FROM storage_quota_folders f JOIN storage_quota q ON q.scope_type='folder' AND q.scope_id=f.id::text WHERE f.share_id=$1 ORDER BY f.path`, shareID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []QuotaFolder{}
	for rows.Next() {
		var f QuotaFolder
		if e = rows.Scan(&f.ID, &f.ShareID, &f.Path, &f.Version, &f.Usage.LimitBytes, &f.Usage.UsedBytes, &f.Usage.ReservedBytes, &f.Usage.Measured); e != nil {
			return nil, e
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

// CreateQuotaFolder registers a folder of an ordinary Shared Folder (not the homes share). Files creates the
// directory if it is missing, or adopts an existing one, and measures it before the usage is trusted.
func (m Manager) CreateQuotaFolder(ctx context.Context, actor, shareID, path string, limit *int64) (QuotaFolder, error) {
	if !uuidRE.MatchString(shareID) {
		return QuotaFolder{}, ErrNotFound
	}
	if !ValidFolderPath(path) || !validLimit(limit) {
		return QuotaFolder{}, ErrInvalid
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return QuotaFolder{}, e
	}
	defer tx.Rollback(ctx)
	var kind string
	if e = tx.QueryRow(ctx, "SELECT kind FROM storage_shares WHERE id=$1 FOR UPDATE", shareID).Scan(&kind); errors.Is(e, pgx.ErrNoRows) {
		return QuotaFolder{}, ErrNotFound
	} else if e != nil {
		return QuotaFolder{}, e
	}
	if kind != "share" {
		return QuotaFolder{}, ErrInvalid // homes are private; their quota is the home quota
	}
	var n int
	if e = tx.QueryRow(ctx, "SELECT count(*) FROM storage_quota_folders WHERE share_id=$1", shareID).Scan(&n); e != nil {
		return QuotaFolder{}, e
	}
	if n >= MaxQuotaFolders {
		return QuotaFolder{}, ErrInvalid
	}
	f := QuotaFolder{ShareID: shareID, Path: path, Version: 1, Usage: Usage{LimitBytes: limit}}
	e = tx.QueryRow(ctx, "INSERT INTO storage_quota_folders(share_id,path) VALUES($1,$2) ON CONFLICT DO NOTHING RETURNING id::text", shareID, path).Scan(&f.ID)
	if errors.Is(e, pgx.ErrNoRows) {
		return QuotaFolder{}, ErrExists
	}
	if e != nil {
		return QuotaFolder{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO storage_quota(scope_type,scope_id,limit_bytes,measure_requested) VALUES('folder',$1,$2,true)", f.ID, limit); e != nil {
		return QuotaFolder{}, e
	}
	if e = event(ctx, tx, actor, "storage.folder.created", f.ID, map[string]any{"share": shareID, "limitBytes": limit}); e != nil {
		return QuotaFolder{}, e
	}
	return f, tx.Commit(ctx)
}

func (m Manager) SetQuotaFolderLimit(ctx context.Context, actor, id string, version int64, limit *int64) error {
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
	tag, e := tx.Exec(ctx, "UPDATE storage_quota SET limit_bytes=$2,version=version+1 WHERE scope_type='folder' AND scope_id=$1 AND version=$3", id, limit, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		var exists bool
		tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_quota_folders WHERE id=$1)", id).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
		return ErrConflict
	}
	if e = event(ctx, tx, actor, "storage.folder.quota", id, map[string]any{"limitBytes": limit}); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// DeleteQuotaFolder removes the registration and its ledger only; the folder and its files stay.
func (m Manager) DeleteQuotaFolder(ctx context.Context, actor, id string, version int64) error {
	if !uuidRE.MatchString(id) {
		return ErrNotFound
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "DELETE FROM storage_quota WHERE scope_type='folder' AND scope_id=$1 AND version=$2", id, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		var exists bool
		tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM storage_quota_folders WHERE id=$1)", id).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
		return ErrConflict
	}
	// Audit first: the event resolves the organization through the folder registration.
	if e = event(ctx, tx, actor, "storage.folder.removed", id, nil); e != nil {
		return e
	}
	if _, e = tx.Exec(ctx, "DELETE FROM storage_quota_folders WHERE id=$1", id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// RequestShareMeasure asks Files to re-measure an ordinary share (all its files and trash) in the background.
// The homes share total is the sum of the homes and is repaired by measuring homes.
func (m Manager) RequestShareMeasure(ctx context.Context, actor, shareID string) error {
	if !uuidRE.MatchString(shareID) {
		return ErrNotFound
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var kind string
	if e = tx.QueryRow(ctx, "SELECT kind FROM storage_shares WHERE id=$1", shareID).Scan(&kind); errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if kind != "share" {
		return ErrInvalid
	}
	if _, e = tx.Exec(ctx, "UPDATE storage_quota SET measure_requested=true WHERE scope_type='share' AND scope_id=$1", shareID); e != nil {
		return e
	}
	if e = event(ctx, tx, actor, "storage.share.measure", shareID, nil); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// FolderScope is what Files needs to place a write: the folder ID and its path in the share.
type FolderScope struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

func (s Store) ShareFolders(ctx context.Context, shareID string) ([]FolderScope, error) {
	rows, e := s.DB.Query(ctx, "SELECT id::text,path FROM storage_quota_folders WHERE share_id=$1 ORDER BY path", shareID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []FolderScope{}
	for rows.Next() {
		var f FolderScope
		if e = rows.Scan(&f.ID, &f.Path); e != nil {
			return nil, e
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

// MoveFolders rewrites registered paths after Files renamed or moved `from` to `to` inside a share: the
// folder itself and every quota folder below it keep their IDs and usage.
func (s Store) MoveFolders(ctx context.Context, shareID, from, to string) error {
	if !ValidFolderPath(from) || !ValidFolderPath(to) {
		return ErrInvalid
	}
	_, e := s.DB.Exec(ctx, `UPDATE storage_quota_folders SET path=$3||substr(path,length($2)+1)
		WHERE share_id=$1 AND (path=$2 OR left(path,length($2)+1)=$2||'/')`, shareID, from, to)
	return e
}

// AdjustFolders moves usage between quota folders without a limit check: data left the folders (negative
// delta: trash, move out) or came back from the trash (restore is never refused).
func (s Store) AdjustFolders(ctx context.Context, folders []string, delta int64) error {
	if len(folders) == 0 || delta == 0 {
		return nil
	}
	_, e := s.DB.Exec(ctx, "UPDATE storage_quota SET used_bytes=greatest(0,used_bytes+$2) WHERE scope_type='folder' AND scope_id=ANY($1)", folders, delta)
	return e
}

// PendingFolder is a quota folder Files must create/adopt and measure.
type PendingFolder struct {
	ID, ShareID, ShareKey, Volume, Path string
}

func (s Store) PendingFolderMeasures(ctx context.Context, limit int) ([]PendingFolder, error) {
	rows, e := s.DB.Query(ctx, `SELECT f.id::text,s.id::text,s.key,s.volume,f.path FROM storage_quota q JOIN storage_quota_folders f ON f.id::text=q.scope_id
		JOIN storage_shares s ON s.id=f.share_id WHERE q.scope_type='folder' AND q.measure_requested ORDER BY f.created_at LIMIT $1`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []PendingFolder{}
	for rows.Next() {
		var p PendingFolder
		if e = rows.Scan(&p.ID, &p.ShareID, &p.ShareKey, &p.Volume, &p.Path); e != nil {
			return nil, e
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s Store) SetFolderMeasured(ctx context.Context, id string, bytes int64) error {
	_, e := s.DB.Exec(ctx, "UPDATE storage_quota SET used_bytes=$2,measure_requested=false,measured_at=now() WHERE scope_type='folder' AND scope_id=$1", id, bytes)
	return e
}

// PendingShare is an ordinary share whose usage an admin asked to re-measure.
type PendingShare struct {
	ID, Key, Volume string
}

func (s Store) PendingShareMeasures(ctx context.Context, limit int) ([]PendingShare, error) {
	rows, e := s.DB.Query(ctx, `SELECT s.id::text,s.key,s.volume FROM storage_quota q JOIN storage_shares s ON s.id::text=q.scope_id
		WHERE q.scope_type='share' AND q.measure_requested AND s.kind='share' ORDER BY s.key LIMIT $1`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []PendingShare{}
	for rows.Next() {
		var p PendingShare
		if e = rows.Scan(&p.ID, &p.Key, &p.Volume); e != nil {
			return nil, e
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s Store) SetShareMeasured(ctx context.Context, id string, bytes int64) error {
	_, e := s.DB.Exec(ctx, "UPDATE storage_quota SET used_bytes=$2,measure_requested=false,measured_at=now() WHERE scope_type='share' AND scope_id=$1", id, bytes)
	return e
}

// RequestShareMeasure flags a share for the background measurer (after data left it outside the ledger, e.g. an
// emptied trash).
func (s Store) RequestShareMeasure(ctx context.Context, id string) error {
	_, e := s.DB.Exec(ctx, "UPDATE storage_quota SET measure_requested=true WHERE scope_type='share' AND scope_id=$1", id)
	return e
}

// QuotaFolderUsage is the read side Files shows inside a share (same rows as Storage Manager).
func (s Store) QuotaFolderUsage(ctx context.Context, shareID string) ([]QuotaFolder, error) {
	return Manager{Store: s}.QuotaFolders(ctx, shareID)
}
