package drive

import (
	"context"
	"errors"
	"path"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/authn"
)

// Sharing (ADR 0011, files SQL 008): grants share an item of the owner's home with users of a common organization;
// links share it with anyone holding the URL. Limits keep one account from filling the tables.
const (
	MaxGrants = 1000
	MaxLinks  = 500
)

var errNoRecipient = errors.New("recipient not reachable")

// Grant is an item shared with one user. OwnerID and GranteeID are internal account IDs (never sent to clients);
// clients see the public IDs and names.
type Grant struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Dir         bool      `json:"dir"`
	Permission  string    `json:"permission"`
	Owner       string    `json:"owner"`
	Grantee     string    `json:"granteeId"`
	GranteeName string    `json:"grantee"`
	Created     time.Time `json:"created"`
	OwnerID     string    `json:"-"`
	GranteeID   string    `json:"-"`
}

// Link is a public link; Token is the secret part of the URL /s/<token>.
type Link struct {
	ID          string     `json:"id"`
	Token       string     `json:"token"`
	Path        string     `json:"path"`
	Name        string     `json:"name"`
	Dir         bool       `json:"dir"`
	Permission  string     `json:"permission"`
	HasPassword bool       `json:"hasPassword"`
	Expires     *time.Time `json:"expires"`
	Created     time.Time  `json:"created"`
	LastUsed    *time.Time `json:"lastUsed"`
	Uses        int64      `json:"uses"`
	OwnerID     string     `json:"-"`
	Owner       string     `json:"-"`
	hash        string
}

// Recipient is a user the owner may share with (same active organization).
type Recipient struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"displayName"`
}

// SharingStore keeps grants and links; nil disables sharing (tests, standalone without a database).
type SharingStore interface {
	Recipients(ctx context.Context, user, query string) ([]Recipient, error)
	Grants(ctx context.Context, owner, path string) ([]Grant, error)
	Received(ctx context.Context, user string) ([]Grant, error)
	// ReceivedGrant finds a grant to user that is still valid (both accounts active, a common organization).
	ReceivedGrant(ctx context.Context, id, user string) (Grant, bool, error)
	AddGrant(ctx context.Context, owner string, g Grant) (Grant, error)
	RemoveGrant(ctx context.Context, owner, id string) (bool, error)
	Links(ctx context.Context, owner, path string) ([]Link, error)
	AddLink(ctx context.Context, owner string, l Link, passwordHash string) (Link, error)
	UpdateLink(ctx context.Context, owner string, l Link, passwordHash *string) (Link, bool, error)
	RemoveLink(ctx context.Context, owner, id string) (bool, error)
	LinkByToken(ctx context.Context, token string) (Link, bool, error)
	UsedLink(ctx context.Context, id string) error
	// Retarget follows a rename or move of an item (to empty: the item was trashed or deleted) in the owner's
	// shares and favorites, and in every user's favorites of a Shared Folder (root share:<key>, owner empty).
	Retarget(ctx context.Context, owner, root, from, to string) error
}

type SQLSharing struct{ DB *pgxpool.Pool }

const grantColumns = `g.id,g.path,g.dir,g.permission,o.display_name,r.public_id::text,r.display_name,g.created_at,g.owner_id,g.grantee_id
	FROM files_grants g JOIN users o ON o.id=g.owner_id JOIN users r ON r.id=g.grantee_id`

func scanGrants(rows pgx.Rows, err error) ([]Grant, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Grant, error) {
		var g Grant
		err := r.Scan(&g.ID, &g.Path, &g.Dir, &g.Permission, &g.Owner, &g.Grantee, &g.GranteeName, &g.Created, &g.OwnerID, &g.GranteeID)
		g.Name = path.Base(g.Path)
		return g, err
	})
}

// reachable: both accounts active and members of a common active organization.
const reachable = `o.status='active' AND r.status='active' AND EXISTS (SELECT 1 FROM identity_memberships a
	JOIN identity_memberships b ON b.organization_id=a.organization_id JOIN identity_organizations org ON org.id=a.organization_id
	WHERE a.user_id=o.public_id AND b.user_id=r.public_id AND org.status='active')`

func (s SQLSharing) Recipients(ctx context.Context, user, query string) ([]Recipient, error) {
	rows, err := s.DB.Query(ctx, `SELECT r.public_id::text,r.username,r.display_name FROM users r JOIN users o ON o.id=$1
		WHERE r.id<>o.id AND `+reachable+` AND (r.username ILIKE '%'||$2||'%' OR r.display_name ILIKE '%'||$2||'%')
		ORDER BY r.display_name, r.username LIMIT 20`, user, escapeLike(query))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Recipient])
}

func (s SQLSharing) Grants(ctx context.Context, owner, path string) ([]Grant, error) {
	return scanGrants(s.DB.Query(ctx, "SELECT "+grantColumns+" WHERE g.owner_id=$1 AND ($2='' OR g.path=$2) ORDER BY g.path, r.display_name", owner, path))
}

func (s SQLSharing) Received(ctx context.Context, user string) ([]Grant, error) {
	return scanGrants(s.DB.Query(ctx, "SELECT "+grantColumns+" WHERE g.grantee_id=$1 AND "+reachable+" ORDER BY g.created_at DESC", user))
}

func (s SQLSharing) ReceivedGrant(ctx context.Context, id, user string) (Grant, bool, error) {
	list, err := scanGrants(s.DB.Query(ctx, "SELECT "+grantColumns+" WHERE g.id=$1 AND g.grantee_id=$2 AND "+reachable, id, user))
	if err != nil || len(list) == 0 {
		return Grant{}, false, err
	}
	return list[0], true, nil
}

// AddGrant shares with the recipient (public ID), or changes the permission of an existing grant.
func (s SQLSharing) AddGrant(ctx context.Context, owner string, g Grant) (Grant, error) {
	var id string
	err := s.DB.QueryRow(ctx, `INSERT INTO files_grants(id,owner_id,path,dir,grantee_id,permission)
		SELECT $2,o.id,$3,$4,r.id,$6 FROM users o JOIN users r ON r.public_id::text=$5 WHERE o.id=$1 AND r.id<>o.id AND `+reachable+`
		AND ((SELECT count(*) FROM files_grants WHERE owner_id=$1) < $7 OR EXISTS (SELECT 1 FROM files_grants WHERE owner_id=$1 AND path=$3 AND grantee_id=r.id))
		ON CONFLICT (owner_id,path,grantee_id) DO UPDATE SET permission=EXCLUDED.permission, dir=EXCLUDED.dir RETURNING id`,
		owner, authn.Token(), g.Path, g.Dir, g.Grantee, g.Permission, MaxGrants).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Grant{}, errNoRecipient
	}
	if err != nil {
		return Grant{}, err
	}
	list, err := scanGrants(s.DB.Query(ctx, "SELECT "+grantColumns+" WHERE g.id=$1", id))
	if err != nil || len(list) == 0 {
		return Grant{}, errors.Join(err, errNoRecipient)
	}
	return list[0], nil
}

func (s SQLSharing) RemoveGrant(ctx context.Context, owner, id string) (bool, error) {
	tag, err := s.DB.Exec(ctx, "DELETE FROM files_grants WHERE id=$1 AND owner_id=$2", id, owner)
	if err == nil && tag.RowsAffected() > 0 {
		_, err = s.DB.Exec(ctx, "DELETE FROM files_favorites WHERE root=$1", "shared:"+id)
	}
	return err == nil && tag.RowsAffected() > 0, err
}

const linkColumns = `l.id,l.token,l.path,l.dir,l.permission,l.password_hash,l.expires_at,l.created_at,l.last_used_at,l.uses,l.owner_id,o.display_name
	FROM files_links l JOIN users o ON o.id=l.owner_id`

func scanLinks(rows pgx.Rows, err error) ([]Link, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Link, error) {
		var l Link
		var hash *string
		err := r.Scan(&l.ID, &l.Token, &l.Path, &l.Dir, &l.Permission, &hash, &l.Expires, &l.Created, &l.LastUsed, &l.Uses, &l.OwnerID, &l.Owner)
		if hash != nil {
			l.hash, l.HasPassword = *hash, true
		}
		l.Name = path.Base(l.Path)
		return l, err
	})
}

func (s SQLSharing) Links(ctx context.Context, owner, path string) ([]Link, error) {
	return scanLinks(s.DB.Query(ctx, "SELECT "+linkColumns+" WHERE l.owner_id=$1 AND ($2='' OR l.path=$2) ORDER BY l.path, l.created_at", owner, path))
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (s SQLSharing) AddLink(ctx context.Context, owner string, l Link, passwordHash string) (Link, error) {
	var id string
	err := s.DB.QueryRow(ctx, `INSERT INTO files_links(id,token,owner_id,path,dir,permission,password_hash,expires_at)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8 WHERE (SELECT count(*) FROM files_links WHERE owner_id=$3) < $9 RETURNING id`,
		authn.Token(), authn.Token(), owner, l.Path, l.Dir, l.Permission, nullable(passwordHash), l.Expires, MaxLinks).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, problem(409, "FILES_LINKS_LIMIT", "Đã đạt giới hạn liên kết công khai.")
	}
	if err != nil {
		return Link{}, err
	}
	list, err := scanLinks(s.DB.Query(ctx, "SELECT "+linkColumns+" WHERE l.id=$1", id))
	if err != nil || len(list) == 0 {
		return Link{}, errors.Join(err, pgx.ErrNoRows)
	}
	return list[0], nil
}

// UpdateLink changes the permission and expiry; passwordHash nil keeps the password, "" removes it.
func (s SQLSharing) UpdateLink(ctx context.Context, owner string, l Link, passwordHash *string) (Link, bool, error) {
	tag, err := s.DB.Exec(ctx, `UPDATE files_links SET permission=$3, expires_at=$4,
		password_hash=CASE WHEN $5 THEN NULLIF($6,'') ELSE password_hash END WHERE id=$1 AND owner_id=$2 AND (dir OR $3='view')`,
		l.ID, owner, l.Permission, l.Expires, passwordHash != nil, derefString(passwordHash))
	if err != nil || tag.RowsAffected() == 0 {
		return Link{}, false, err
	}
	list, err := scanLinks(s.DB.Query(ctx, "SELECT "+linkColumns+" WHERE l.id=$1", l.ID))
	if err != nil || len(list) == 0 {
		return Link{}, false, err
	}
	return list[0], true, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s SQLSharing) RemoveLink(ctx context.Context, owner, id string) (bool, error) {
	tag, err := s.DB.Exec(ctx, "DELETE FROM files_links WHERE id=$1 AND owner_id=$2", id, owner)
	return err == nil && tag.RowsAffected() > 0, err
}

// LinkByToken finds a link whose owner is still active (expiry is checked by the caller to answer 410).
func (s SQLSharing) LinkByToken(ctx context.Context, token string) (Link, bool, error) {
	list, err := scanLinks(s.DB.Query(ctx, "SELECT "+linkColumns+" WHERE l.token=$1 AND o.status='active'", token))
	if err != nil || len(list) == 0 {
		return Link{}, false, err
	}
	return list[0], true, nil
}

func (s SQLSharing) UsedLink(ctx context.Context, id string) error {
	_, err := s.DB.Exec(ctx, "UPDATE files_links SET uses=uses+1, last_used_at=now() WHERE id=$1", id)
	return err
}

func (s SQLSharing) Retarget(ctx context.Context, owner, root, from, to string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// under: the item itself or anything inside it.
	const under = "(path=$2 OR left(path, length($2)+1)=$2||'/')"
	if owner != "" && root == "personal" {
		// Favorites of people an item is shared with hold paths relative to the shared folder (the name of a shared
		// file): a change inside a shared folder moves them, or removes them when the item leaves the folder.
		const inside = `g.owner_id=$1 AND g.dir AND f.root='shared:'||g.id AND left($2, length(g.path)+1)=g.path||'/'
			AND (f.path=substr($2, length(g.path)+2) OR left(f.path, length($2)-length(g.path))=substr($2, length(g.path)+2)||'/')`
		const stays = "$3<>'' AND left($3, length(g.path)+1)=g.path||'/'"
		if _, err = tx.Exec(ctx, "DELETE FROM files_favorites f USING files_grants g WHERE "+inside+" AND NOT ("+stays+")", owner, from, to); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "UPDATE files_favorites f SET path=substr($3, length(g.path)+2)||substr(f.path, length($2)-length(g.path)) FROM files_grants g WHERE "+inside+" AND "+stays, owner, from, to); err != nil {
			return err
		}
		if to != "" {
			if _, err = tx.Exec(ctx, "UPDATE files_favorites f SET path=$3 FROM files_grants g WHERE g.owner_id=$1 AND NOT g.dir AND g.path=$2 AND f.root='shared:'||g.id", owner, from, path.Base(to)); err != nil {
				return err
			}
		}
	}
	if to == "" {
		if owner != "" && root == "personal" {
			if _, err = tx.Exec(ctx, "DELETE FROM files_favorites WHERE root IN (SELECT 'shared:'||id FROM files_grants WHERE owner_id=$1 AND "+under+")", owner, from); err != nil {
				return err
			}
			for _, table := range []string{"files_grants", "files_links"} {
				if _, err = tx.Exec(ctx, "DELETE FROM "+table+" WHERE owner_id=$1 AND "+under, owner, from); err != nil {
					return err
				}
			}
		}
		if _, err = tx.Exec(ctx, "DELETE FROM files_favorites WHERE ($1='' OR user_id=$1) AND root=$3 AND "+under, owner, from, root); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	const moved = "path=$3||substr(path, length($2)+1)"
	if owner != "" && root == "personal" {
		for _, table := range []string{"files_grants", "files_links"} {
			if _, err = tx.Exec(ctx, "UPDATE "+table+" SET "+moved+" WHERE owner_id=$1 AND "+under, owner, from, to); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(ctx, "UPDATE files_favorites SET "+moved+" WHERE ($1='' OR user_id=$1) AND root=$4 AND "+under, owner, from, to, root); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func escapeLike(s string) string {
	out := make([]rune, 0, len(s))
	for _, c := range s {
		if c == '%' || c == '_' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}
