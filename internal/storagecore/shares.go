package storagecore

import (
	"context"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrNoHomes: no share hosts user homes yet (fresh install, first-run rule). Files has no personal space.
	ErrNoHomes = errors.New("no share hosts user homes")
	// ErrHomesExists: only one share may host homes; moving homes to another share is a later job.
	ErrHomesExists = errors.New("a share already hosts user homes")
	uuidRE         = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// MaxAccessEntries bounds one share's permission list.
const MaxAccessEntries = 200

// HomesShare is where personal homes live. VolumeKind "directory" is the legacy Files data directory
// (homes at DataDir/users/<id>); on ZFS/Btrfs each home is its own unit homes/<id> on the volume.
type HomesShare struct {
	ID, Key, Volume, VolumeKind string
}

func (s Store) HomesShare(ctx context.Context) (HomesShare, error) {
	var h HomesShare
	e := s.DB.QueryRow(ctx, "SELECT s.id::text,s.key,s.volume,v.kind FROM storage_shares s JOIN storage_volumes v ON v.name=s.volume WHERE s.kind='homes'").Scan(&h.ID, &h.Key, &h.Volume, &h.VolumeKind)
	if errors.Is(e, pgx.ErrNoRows) {
		return h, ErrNoHomes
	}
	return h, e
}

// UserShare is a Shared Folder a user may open in Files with the effective access (read or write).
type UserShare struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Name   string `json:"name"`
	Volume string `json:"-"`
	Access string `json:"access"`
}

// UserShares resolves the user's effective access per share: entries for the user and their groups;
// deny wins, then write, then read. Shares without an entry are not listed.
func (s Store) UserShares(ctx context.Context, storageID string) ([]UserShare, error) {
	rows, e := s.DB.Query(ctx, `WITH me AS (SELECT public_id FROM users WHERE id=$1 AND status='active')
		SELECT s.id::text,s.key,s.name,s.volume,CASE WHEN bool_or(a.access='deny') THEN 'deny' WHEN bool_or(a.access='write') THEN 'write' ELSE 'read' END
		FROM storage_shares s JOIN storage_share_access a ON a.share_id=s.id, me
		WHERE s.kind='share' AND ((a.subject_type='user' AND a.subject_id=me.public_id) OR (a.subject_type='group' AND a.subject_id IN (SELECT group_id FROM identity_group_members WHERE user_id=me.public_id)))
		GROUP BY 1,2,3,4 ORDER BY 3`, storageID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []UserShare{}
	for rows.Next() {
		var u UserShare
		if e = rows.Scan(&u.ID, &u.Key, &u.Name, &u.Volume, &u.Access); e != nil {
			return nil, e
		}
		if u.Access != "deny" {
			result = append(result, u)
		}
	}
	return result, rows.Err()
}

// RetireEmptyLegacyHomes applies the first-run rule on a fresh install: storage SQL 001 registered the old
// Files data directory as the homes share; when that directory holds no user data and no home has usage, the
// registration is removed so the first System Admin creates the share hosting homes. Upgrades keep it.
func (s Store) RetireEmptyLegacyHomes(ctx context.Context, hasData bool) (bool, error) {
	if hasData {
		return false, nil
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return false, e
	}
	defer tx.Rollback(ctx)
	var id string
	e = tx.QueryRow(ctx, `SELECT s.id::text FROM storage_shares s JOIN storage_volumes v ON v.name=s.volume WHERE s.kind='homes' AND v.kind='directory'
		AND NOT EXISTS(SELECT 1 FROM storage_quota WHERE scope_type='home' AND used_bytes>0) FOR UPDATE OF s`).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	for _, q := range []string{
		"DELETE FROM storage_reservations WHERE share_id=$1",
		"DELETE FROM storage_quota WHERE (scope_type='share' AND scope_id=$1) OR scope_type='home'",
		"DELETE FROM storage_shares WHERE id::text=$1",
	} {
		if _, e = tx.Exec(ctx, q, id); e != nil {
			return false, e
		}
	}
	if _, e = tx.Exec(ctx, "DELETE FROM storage_volumes v WHERE kind='directory' AND NOT EXISTS(SELECT 1 FROM storage_shares s WHERE s.volume=v.name)"); e != nil {
		return false, e
	}
	if e = event(ctx, tx, "", "storage.homes.legacy_retired", id, nil); e != nil {
		return false, e
	}
	return true, tx.Commit(ctx)
}

// AccessEntry is one permission of a share. Name is filled on reads (username or group name).
type AccessEntry struct {
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
	Name        string `json:"name,omitempty"`
	Access      string `json:"access"`
}

type AccessList struct {
	Version int64         `json:"version"`
	Entries []AccessEntry `json:"entries"`
}

func (m Manager) ShareAccess(ctx context.Context, shareID string) (AccessList, error) {
	if !uuidRE.MatchString(shareID) {
		return AccessList{}, ErrNotFound
	}
	list := AccessList{Entries: []AccessEntry{}}
	var kind string
	if e := m.Store.DB.QueryRow(ctx, "SELECT version,kind FROM storage_shares WHERE id=$1", shareID).Scan(&list.Version, &kind); errors.Is(e, pgx.ErrNoRows) {
		return list, ErrNotFound
	} else if e != nil {
		return list, e
	}
	if kind != "share" {
		return list, ErrInvalid // homes are private to their owners; no share permissions
	}
	rows, e := m.Store.DB.Query(ctx, `SELECT a.subject_type,a.subject_id::text,coalesce(u.username,g.name,''),a.access FROM storage_share_access a
		LEFT JOIN users u ON a.subject_type='user' AND u.public_id=a.subject_id
		LEFT JOIN identity_groups g ON a.subject_type='group' AND g.id=a.subject_id
		WHERE a.share_id=$1 ORDER BY a.subject_type DESC,3`, shareID)
	if e != nil {
		return list, e
	}
	defer rows.Close()
	for rows.Next() {
		var a AccessEntry
		if e = rows.Scan(&a.SubjectType, &a.SubjectID, &a.Name, &a.Access); e != nil {
			return list, e
		}
		list.Entries = append(list.Entries, a)
	}
	return list, rows.Err()
}

// SetShareAccess replaces a share's permission list (If-Match on the share version). Every subject must be an
// existing account or group; the change is audited with the counts, not the list.
func (m Manager) SetShareAccess(ctx context.Context, actor, shareID string, version int64, entries []AccessEntry) error {
	if !uuidRE.MatchString(shareID) {
		return ErrNotFound
	}
	if len(entries) > MaxAccessEntries {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, a := range entries {
		key := a.SubjectType + ":" + a.SubjectID
		if (a.SubjectType != "user" && a.SubjectType != "group") || !uuidRE.MatchString(a.SubjectID) || (a.Access != "read" && a.Access != "write" && a.Access != "deny") || seen[key] {
			return ErrInvalid
		}
		seen[key] = true
	}
	tx, e := m.Store.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var kind string
	if e = tx.QueryRow(ctx, "SELECT kind FROM storage_shares WHERE id=$1 FOR UPDATE", shareID).Scan(&kind); errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	} else if e != nil {
		return e
	}
	if kind != "share" {
		return ErrInvalid
	}
	tag, e := tx.Exec(ctx, "UPDATE storage_shares SET version=version+1 WHERE id=$1 AND version=$2", shareID, version)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if _, e = tx.Exec(ctx, "DELETE FROM storage_share_access WHERE share_id=$1", shareID); e != nil {
		return e
	}
	counts := map[string]any{"read": 0, "write": 0, "deny": 0}
	for _, a := range entries {
		var ok bool
		q := "SELECT EXISTS(SELECT 1 FROM users WHERE public_id=$1 AND status<>'deleted')"
		if a.SubjectType == "group" {
			q = "SELECT EXISTS(SELECT 1 FROM identity_groups WHERE id=$1)"
		}
		if e = tx.QueryRow(ctx, q, a.SubjectID).Scan(&ok); e != nil {
			return e
		}
		if !ok {
			return ErrInvalid
		}
		if _, e = tx.Exec(ctx, "INSERT INTO storage_share_access(share_id,subject_type,subject_id,access) VALUES($1,$2,$3,$4)", shareID, a.SubjectType, a.SubjectID, a.Access); e != nil {
			return e
		}
		counts[a.Access] = counts[a.Access].(int) + 1
	}
	if e = event(ctx, tx, actor, "storage.share.access", shareID, counts); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

// Subject is an account or group that can receive share permissions.
type Subject struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

// Subjects lists active accounts and all groups (bounded) for the permission editor.
func (m Manager) Subjects(ctx context.Context) ([]Subject, error) {
	rows, e := m.Store.DB.Query(ctx, `(SELECT 'user',public_id::text,username,display_name FROM users WHERE status='active' AND public_id IS NOT NULL ORDER BY username LIMIT 500)
		UNION ALL (SELECT 'group',id::text,name,key FROM identity_groups ORDER BY name LIMIT 500)`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	result := []Subject{}
	for rows.Next() {
		var s Subject
		if e = rows.Scan(&s.Type, &s.ID, &s.Name, &s.Detail); e != nil {
			return nil, e
		}
		result = append(result, s)
	}
	return result, rows.Err()
}
