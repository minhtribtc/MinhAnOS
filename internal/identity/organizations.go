package identity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"minhanos.local/platform/internal/authn"
)

// Organizations (ADR 0008). Authority is relationship-based: identity_org_role per request, System Admin is
// "root". An organization the caller has no role in reads as not found, so IDs cannot be probed.

const maxOrgDepth = 8

type Organization struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ParentID      *string `json:"parentId"`
	Depth         int     `json:"depth"`
	Status        string  `json:"status"`
	AllowChildren bool    `json:"allowChildren"`
	IsDefault     bool    `json:"isDefault"`
	// Role is the caller's authority: root, admin, manager or member.
	Role    string `json:"role"`
	Members int    `json:"memberCount"`
	Version int64  `json:"version"`
}

type Member struct {
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	Role        string `json:"role"`
}

type OrgInput struct {
	Name          string  `json:"name"`
	ParentID      *string `json:"parentId"`
	AllowChildren bool    `json:"allowChildren"`
}
type OrgPatch struct {
	Name          *string `json:"name"`
	Status        *string `json:"status"`
	AllowChildren *bool   `json:"allowChildren"`
}
type MemberInput struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}
type OrgUserInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

const orgColumns = `o.id::text,o.name,o.parent_id::text,o.depth,o.status,o.allow_children,o.is_default,o.version,(SELECT count(*) FROM identity_memberships m WHERE m.organization_id=o.id)`

func scanOrg(row pgx.Row, o *Organization) error {
	return row.Scan(&o.ID, &o.Name, &o.ParentID, &o.Depth, &o.Status, &o.AllowChildren, &o.IsDefault, &o.Version, &o.Members)
}

// orgRole returns the caller's role in an organization; no role (or no such organization) is 404.
func orgRole(ctx context.Context, tx pgx.Tx, a Actor, org string) (string, error) {
	if !uuidPattern.MatchString(org) {
		return "", fault(404, "ORG_NOT_FOUND")
	}
	var exists bool
	var role *string
	if e := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM identity_organizations WHERE id=$1),identity_org_role(nullif($2,'')::uuid,$1)", org, a.PublicID).Scan(&exists, &role); e != nil {
		return "", e
	}
	if !exists {
		return "", fault(404, "ORG_NOT_FOUND")
	}
	if a.Role == "admin" {
		return "root", nil
	}
	if role == nil {
		return "", fault(404, "ORG_NOT_FOUND")
	}
	return *role, nil
}

// needRole authorizes one of the allowed roles; a member asking for admin work is 403.
func needRole(ctx context.Context, tx pgx.Tx, a Actor, org string, allowed ...string) (string, error) {
	role, e := orgRole(ctx, tx, a, org)
	if e != nil {
		return "", e
	}
	for _, r := range allowed {
		if r == role {
			return role, nil
		}
	}
	return "", fault(403, "ORG_PERMISSION_DENIED")
}

func readOrg(ctx context.Context, tx pgx.Tx, a Actor, id string) (Organization, error) {
	role, e := orgRole(ctx, tx, a, id)
	if e != nil {
		return Organization{}, e
	}
	var o Organization
	if e = scanOrg(tx.QueryRow(ctx, "SELECT "+orgColumns+" FROM identity_organizations o WHERE o.id=$1", id), &o); e != nil {
		return o, noRows(e)
	}
	o.Role = role
	return o, nil
}

func pgCode(e error) string {
	var pe *pgconn.PgError
	if errors.As(e, &pe) {
		return pe.Code
	}
	return ""
}

// savepoint runs one statement that may fail on a constraint without aborting the transaction.
func savepoint(ctx context.Context, tx pgx.Tx, q string, args ...any) (pgconn.CommandTag, error) {
	sp, e := tx.Begin(ctx)
	if e != nil {
		return pgconn.CommandTag{}, e
	}
	tag, e := sp.Exec(ctx, q, args...)
	if e != nil {
		sp.Rollback(ctx)
		return tag, e
	}
	return tag, sp.Commit(ctx)
}

func validOrgName(name string) bool {
	n := len([]rune(strings.TrimSpace(name)))
	return n >= 1 && n <= 100
}

// visibleOrgs lists the organizations the caller has a role in (Root: all), ordered by id for paging.
func visibleOrgs(ctx context.Context, tx pgx.Tx, a Actor, after string, limit int) ([]Organization, error) {
	rows, e := tx.Query(ctx, `SELECT `+orgColumns+`,CASE WHEN $1 THEN 'root' ELSE identity_org_role(nullif($2,'')::uuid,o.id) END r FROM identity_organizations o
		WHERE o.id::text>$3 AND ($1 OR identity_org_role(nullif($2,'')::uuid,o.id) IS NOT NULL) ORDER BY o.id::text LIMIT $4`, a.Role == "admin", a.PublicID, after, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Organization{}
	for rows.Next() {
		var o Organization
		if e = rows.Scan(&o.ID, &o.Name, &o.ParentID, &o.Depth, &o.Status, &o.AllowChildren, &o.IsDefault, &o.Version, &o.Members, &o.Role); e != nil {
			return nil, e
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Organizations lists the organizations of an account with its role in each (Storage Manager, Log Center,
// launcher). At most 500; storage and log views only need the ones the account administers.
func (s *Service) Organizations(ctx context.Context, storageID string) ([]Organization, error) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	a, e := actor(ctx, tx, storageID)
	if e != nil {
		return nil, e
	}
	list, e := visibleOrgs(ctx, tx, a, "", 500)
	if e != nil {
		return nil, e
	}
	return list, tx.Commit(ctx)
}

// OrganizationRole is the account's role in one organization ("" when none or not found).
func (s *Service) OrganizationRole(ctx context.Context, storageID, org string) (string, error) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	a, e := actor(ctx, tx, storageID)
	if e != nil {
		return "", e
	}
	role, e := orgRole(ctx, tx, a, org)
	var f *Fault
	if errors.As(e, &f) {
		return "", nil
	}
	return role, e
}

// orgRead serves GET organizations, organizations/{id}, …/members, …/groups and …/groups/{g}/members.
func (s *Service) orgRead(ctx context.Context, tx pgx.Tx, a Actor, parts []string, page string, limit int) (Result, error) {
	route := strings.Join(parts, "/")
	after, e := s.decodeCursor(page, a, route, "")
	if e != nil {
		return Result{}, e
	}
	var rows pgx.Rows
	switch {
	case len(parts) == 1:
		list, e := visibleOrgs(ctx, tx, a, after, limit+1)
		if e != nil {
			return Result{}, e
		}
		items := []any{}
		last := ""
		for i, o := range list {
			if i == limit {
				break
			}
			items, last = append(items, o), o.ID
		}
		return s.page(a, route, items, len(list) > limit, last), tx.Commit(ctx)
	case len(parts) == 2:
		o, e := readOrg(ctx, tx, a, parts[1])
		if e != nil {
			return Result{}, e
		}
		return result(o, 200, o.Version), tx.Commit(ctx)
	case len(parts) == 3 && parts[2] == "members":
		if _, e = needRole(ctx, tx, a, parts[1], "root", "admin", "manager"); e != nil {
			return Result{}, e
		}
		rows, e = tx.Query(ctx, `SELECT u.public_id::text,jsonb_build_object('userId',u.public_id,'username',u.username,'displayName',u.display_name,'status',u.status,'role',m.role)
			FROM identity_memberships m JOIN users u ON u.public_id=m.user_id WHERE m.organization_id=$1 AND u.public_id::text>$2 ORDER BY u.public_id::text LIMIT $3`, parts[1], after, limit+1)
	case len(parts) == 3 && parts[2] == "groups":
		if _, e = needRole(ctx, tx, a, parts[1], "root", "admin"); e != nil {
			return Result{}, e
		}
		rows, e = tx.Query(ctx, `SELECT g.id::text,jsonb_build_object('id',g.id,'key',g.key,'name',g.name,'kind',g.kind,'organizationId',g.organization_id,'version',g.version,'memberCount',(SELECT count(*) FROM identity_group_members x WHERE x.group_id=g.id))
			FROM identity_groups g WHERE g.organization_id=$1 AND g.id::text>$2 ORDER BY g.id::text LIMIT $3`, parts[1], after, limit+1)
	case len(parts) == 5 && parts[2] == "groups" && parts[4] == "members":
		if _, e = needRole(ctx, tx, a, parts[1], "root", "admin"); e != nil {
			return Result{}, e
		}
		if _, e = orgGroup(ctx, tx, parts[1], parts[3]); e != nil {
			return Result{}, e
		}
		rows, e = tx.Query(ctx, `SELECT x.user_id::text,jsonb_build_object('userId',x.user_id) FROM identity_group_members x WHERE x.group_id=$1 AND x.user_id::text>$2 ORDER BY x.user_id::text LIMIT $3`, parts[3], after, limit+1)
	default:
		return Result{}, fault(404, "IDENTITY_NOT_FOUND")
	}
	if e != nil {
		return Result{}, e
	}
	defer rows.Close()
	items := []any{}
	last, more := "", false
	for rows.Next() {
		var key string
		var raw json.RawMessage
		if e = rows.Scan(&key, &raw); e != nil {
			return Result{}, e
		}
		if len(items) == limit {
			more = true
			break
		}
		items, last = append(items, raw), key
	}
	if e = rows.Err(); e != nil {
		return Result{}, e
	}
	rows.Close()
	return s.page(a, route, items, more, last), tx.Commit(ctx)
}

func (s *Service) page(a Actor, route string, items []any, more bool, last string) Result {
	var next any
	if more {
		next = s.encodeCursor(cursor{a.PublicID, route, "", last, a.Epoch, time.Now().Add(15 * time.Minute).Unix()})
	}
	return result(map[string]any{"items": items, "nextCursor": next}, 200, 0)
}

func orgGroup(ctx context.Context, tx pgx.Tx, org, id string) (Group, error) {
	g, e := readGroup(ctx, tx, id)
	if e != nil {
		return g, e
	}
	var owner *string
	if e = tx.QueryRow(ctx, "SELECT organization_id::text FROM identity_groups WHERE id=$1", id).Scan(&owner); e != nil {
		return g, e
	}
	if owner == nil || *owner != org {
		return g, fault(404, "IDENTITY_NOT_FOUND")
	}
	return g, nil
}

func orgNotice(ctx context.Context, tx pgx.Tx, user, kind, org string, extra map[string]any) error {
	storage, e := storageIDOf(ctx, tx, user)
	if e != nil {
		return noRows(e)
	}
	var name string
	if e = tx.QueryRow(ctx, "SELECT name FROM identity_organizations WHERE id=$1", org).Scan(&name); e != nil {
		return e
	}
	details := map[string]any{"organization": name}
	for k, v := range extra {
		details[k] = v
	}
	return notify(ctx, tx, storage, kind, "info", false, details)
}

// CreateOrg creates a top-level organization (Root) or a child of an organization that allows children
// (its admin, a manager of it, or Root).
func (s *Service) CreateOrg(ctx context.Context, realm Realm, token, key, requestID string, input OrgInput, raw []byte) (Result, error) {
	return s.mutate(ctx, realm, token, "", requestID, key, "POST /api/v1/identity/organizations", raw, true, func(tx pgx.Tx, a Actor) (Result, error) {
		if !validOrgName(input.Name) {
			return Result{}, fault(422, "ORG_INVALID")
		}
		if input.ParentID == nil {
			if a.Role != "admin" {
				return Result{}, fault(403, "ORG_PERMISSION_DENIED")
			}
		} else {
			parent, e := readOrg(ctx, tx, a, *input.ParentID)
			if e != nil {
				return Result{}, e
			}
			if parent.Role == "member" {
				return Result{}, fault(403, "ORG_PERMISSION_DENIED")
			}
			if !parent.AllowChildren {
				return Result{}, fault(409, "ORG_CHILDREN_NOT_ALLOWED")
			}
			if parent.Depth >= maxOrgDepth {
				return Result{}, fault(409, "ORG_TOO_DEEP")
			}
		}
		var id string
		e := tx.QueryRow(ctx, "INSERT INTO identity_organizations(name,parent_id,allow_children) VALUES($1,$2,$3) ON CONFLICT DO NOTHING RETURNING id::text", strings.TrimSpace(input.Name), input.ParentID, input.AllowChildren).Scan(&id)
		if errors.Is(e, pgx.ErrNoRows) {
			return Result{}, fault(409, "ORG_NAME_EXISTS")
		}
		if e != nil {
			return Result{}, e
		}
		o, e := readOrg(ctx, tx, a, id)
		if e != nil {
			return Result{}, e
		}
		out := result(o, 201, o.Version)
		out.Org, out.Target = id, id
		return out, nil
	})
}

// PatchOrg renames (admin, manager, Root) or changes status / allowChildren (manager or Root only, so an
// admin cannot widen their own authority or lift a suspension).
func (s *Service) PatchOrg(ctx context.Context, realm Realm, token, id, match, requestID string, input OrgPatch, raw []byte) (Result, error) {
	return s.mutate(ctx, realm, token, "", requestID, "", "PATCH /api/v1/identity/organizations/"+id, append([]byte(match+"\x00"), raw...), true, func(tx pgx.Tx, a Actor) (Result, error) {
		o, e := readOrg(ctx, tx, a, id)
		if e != nil {
			return Result{}, e
		}
		allowed := []string{"root", "manager"}
		if input.Status == nil && input.AllowChildren == nil {
			allowed = append(allowed, "admin")
		}
		if _, e = needRole(ctx, tx, a, id, allowed...); e != nil {
			return Result{}, e
		}
		if e = requireVersion(o.Version, match); e != nil {
			return Result{}, e
		}
		if input.Name == nil && input.Status == nil && input.AllowChildren == nil {
			return Result{}, fault(400, "INVALID_REQUEST")
		}
		if input.Name != nil {
			if !validOrgName(*input.Name) {
				return Result{}, fault(422, "ORG_INVALID")
			}
			o.Name = strings.TrimSpace(*input.Name)
		}
		if input.Status != nil {
			if *input.Status != "active" && *input.Status != "suspended" {
				return Result{}, fault(422, "ORG_INVALID")
			}
			if o.IsDefault && *input.Status == "suspended" {
				return Result{}, fault(409, "ORG_DEFAULT_PROTECTED")
			}
			o.Status = *input.Status
		}
		if input.AllowChildren != nil {
			o.AllowChildren = *input.AllowChildren
		}
		tag, e := savepoint(ctx, tx, "UPDATE identity_organizations SET name=$2,status=$3,allow_children=$4,version=version+1 WHERE id=$1", id, o.Name, o.Status, o.AllowChildren)
		if pgCode(e) == "23505" {
			return Result{}, fault(409, "ORG_NAME_EXISTS")
		}
		if e != nil {
			return Result{}, e
		}
		if tag.RowsAffected() != 1 {
			return Result{}, fault(404, "ORG_NOT_FOUND")
		}
		if o, e = readOrg(ctx, tx, a, id); e != nil {
			return Result{}, e
		}
		out := result(o, 200, o.Version)
		out.Org = id
		return out, nil
	})
}

// DeleteOrg removes an empty organization (no children, shares or allocations); memberships and its groups
// go with it. The default organization stays.
func (s *Service) DeleteOrg(ctx context.Context, realm Realm, token, id, match, key, requestID string, raw []byte) (Result, error) {
	return s.mutate(ctx, realm, token, "", requestID, key, "DELETE /api/v1/identity/organizations/"+id, append([]byte(match+"\x00"), raw...), true, func(tx pgx.Tx, a Actor) (Result, error) {
		o, e := readOrg(ctx, tx, a, id)
		if e != nil {
			return Result{}, e
		}
		if _, e = needRole(ctx, tx, a, id, "root", "manager"); e != nil {
			return Result{}, e
		}
		if e = requireVersion(o.Version, match); e != nil {
			return Result{}, e
		}
		if o.IsDefault {
			return Result{}, fault(409, "ORG_DEFAULT_PROTECTED")
		}
		_, e = savepoint(ctx, tx, "DELETE FROM identity_organizations WHERE id=$1", id)
		if c := pgCode(e); c == "23503" || c == "23001" { // foreign key or RESTRICT
			return Result{}, fault(409, "ORG_NOT_EMPTY")
		}
		if e != nil {
			return Result{}, e
		}
		out := result(map[string]bool{"accepted": true}, 200, 0)
		out.Org = id
		return out, nil
	})
}

// MemberWrite adds an existing account by username, changes a member's role or removes a member. Admins,
// managers and Root may do this; the member is notified.
func (s *Service) MemberWrite(ctx context.Context, realm Realm, token, org, operation, member, key, requestID string, input MemberInput, raw []byte) (Result, error) {
	route := map[string]string{"add": "POST /api/v1/identity/organizations/" + org + "/members", "role": "PUT /api/v1/identity/organizations/" + org + "/members/" + member, "remove": "DELETE /api/v1/identity/organizations/" + org + "/members/" + member}[operation]
	return s.mutate(ctx, realm, token, "", requestID, key, route, raw, true, func(tx pgx.Tx, a Actor) (Result, error) {
		if _, e := needRole(ctx, tx, a, org, "root", "admin", "manager"); e != nil {
			return Result{}, e
		}
		if operation != "remove" && input.Role != "admin" && input.Role != "member" {
			return Result{}, fault(422, "ORG_INVALID_ROLE")
		}
		switch operation {
		case "add":
			var user string
			e := tx.QueryRow(ctx, "SELECT public_id::text FROM users WHERE username=$1 AND status='active' AND public_id IS NOT NULL", normalize(input.Username)).Scan(&user)
			if errors.Is(e, pgx.ErrNoRows) {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			if e != nil {
				return Result{}, e
			}
			tag, e := tx.Exec(ctx, "INSERT INTO identity_memberships(organization_id,user_id,role) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", org, user, input.Role)
			if e != nil {
				return Result{}, e
			}
			if tag.RowsAffected() != 1 {
				return Result{}, fault(409, "ORG_MEMBER_EXISTS")
			}
			member = user
			e = orgNotice(ctx, tx, user, "organization.joined", org, map[string]any{"role": input.Role})
			if e != nil {
				return Result{}, e
			}
		case "role", "remove":
			if !uuidPattern.MatchString(member) {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			q, kind := "UPDATE identity_memberships SET role=$3 WHERE organization_id=$1 AND user_id=$2", "organization.role"
			args := []any{org, member, input.Role}
			if operation == "remove" {
				q, kind, args = "DELETE FROM identity_memberships WHERE organization_id=$1 AND user_id=$2", "organization.left", args[:2]
			}
			tag, e := tx.Exec(ctx, q, args...)
			if e != nil {
				return Result{}, e
			}
			if tag.RowsAffected() != 1 {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			extra := map[string]any{}
			if operation == "role" {
				extra["role"] = input.Role
			}
			if e = orgNotice(ctx, tx, member, kind, org, extra); e != nil {
				return Result{}, e
			}
		default:
			return Result{}, fault(400, "INVALID_REQUEST")
		}
		out := result(map[string]any{"accepted": true, "userId": member}, 200, 0)
		if operation == "add" {
			out.Status = 201
		}
		out.Org, out.Target = org, member
		return out, nil
	})
}

// CreateOrgUser creates a standard account that starts as a member (or admin) of the organization.
func (s *Service) CreateOrgUser(ctx context.Context, realm Realm, token, org, key, requestID string, input OrgUserInput, raw []byte) (Result, error) {
	input.Username = normalize(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if len([]rune(input.Password)) < 12 {
		return Result{}, ErrPasswordPolicy
	}
	if !namePattern.MatchString(input.Username) || len([]rune(input.DisplayName)) < 1 || len([]rune(input.DisplayName)) > 100 {
		return Result{}, fault(422, "IDENTITY_INVALID_USER")
	}
	if input.Role != "admin" && input.Role != "member" {
		return Result{}, fault(422, "ORG_INVALID_ROLE")
	}
	// Authorize before the slow password hash.
	if e := s.orgPreflight(ctx, realm, token, org, "root", "admin", "manager"); e != nil {
		return Result{}, e
	}
	hash, e := s.Auth.PasswordHash(input.Password)
	if e != nil {
		return Result{}, e
	}
	return s.mutate(ctx, realm, token, "", requestID, key, "POST /api/v1/identity/organizations/"+org+"/users", raw, true, func(tx pgx.Tx, a Actor) (Result, error) {
		if _, e := needRole(ctx, tx, a, org, "root", "admin", "manager"); e != nil {
			return Result{}, e
		}
		var u User
		e := tx.QueryRow(ctx, "INSERT INTO users(id,username,display_name,role,password_hash) VALUES($1,$2,$3,'user',$4) ON CONFLICT(username) DO NOTHING RETURNING public_id::text,username,display_name,status,version", authn.Token(), input.Username, input.DisplayName, hash).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Status, &u.Version)
		if errors.Is(e, pgx.ErrNoRows) {
			return Result{}, fault(409, "IDENTITY_USERNAME_EXISTS")
		}
		if e != nil {
			return Result{}, e
		}
		if _, e = tx.Exec(ctx, "INSERT INTO identity_memberships(organization_id,user_id,role) VALUES($1,$2,$3)", org, u.ID, input.Role); e != nil {
			return Result{}, e
		}
		out := result(u, 201, u.Version)
		out.Org, out.Target = org, u.ID
		return out, nil
	})
}

func (s *Service) orgPreflight(ctx context.Context, realm Realm, token, org string, roles ...string) error {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return e
	}
	a, e := actor(ctx, tx, id)
	if e != nil {
		return e
	}
	if _, e = needRole(ctx, tx, a, org, roles...); e != nil {
		return e
	}
	if e = fresh(ctx, tx, realm, token); e != nil {
		return e
	}
	return tx.Commit(ctx)
}

func orgActionName(route string) string {
	method := strings.SplitN(route, " ", 2)[0]
	switch {
	case strings.Contains(route, "/members"):
		return map[string]string{"POST": "org.member.added", "PUT": "org.member.role", "DELETE": "org.member.removed"}[method]
	case strings.HasSuffix(route, "/users"):
		return "org.user.created"
	}
	return map[string]string{"POST": "org.created", "PATCH": "org.updated", "DELETE": "org.deleted"}[method]
}

// ReadOrganizations is the GET side of /api/v1/identity/organizations/… (limit 1..100, signed cursor).
func (s *Service) ReadOrganizations(ctx context.Context, realm Realm, token string, parts []string, page string, limit int) (Result, error) {
	if limit < 1 || limit > 100 {
		return Result{}, fault(400, "INVALID_REQUEST")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return Result{}, e
	}
	defer tx.Rollback(ctx)
	id, e := lockSession(ctx, tx, realm, token)
	if e != nil {
		return Result{}, e
	}
	a, e := actor(ctx, tx, id)
	if e != nil {
		return Result{}, e
	}
	if a.PublicID == "" {
		return Result{}, ErrNotReady
	}
	return s.orgRead(ctx, tx, a, parts, page, limit)
}

// orgWriteHTTP routes the organization writes. POST and DELETE need Idempotency-Key; PATCH/DELETE of an
// organization or group need If-Match.
func (s *Service) orgWriteHTTP(ctx context.Context, realm Realm, token, method string, parts []string, match, key, requestID string, raw []byte, decode func(any) error, empty, needKey func() error) (Result, error) {
	n := len(parts)
	keyed := func(fn func() (Result, error)) (Result, error) {
		if e := needKey(); e != nil {
			return Result{}, e
		}
		return fn()
	}
	switch {
	case n == 1 && method == "POST":
		return keyed(func() (Result, error) {
			var in OrgInput
			if e := decode(&in); e != nil {
				return Result{}, e
			}
			return s.CreateOrg(ctx, realm, token, key, requestID, in, raw)
		})
	case n == 2 && method == "PATCH":
		var in OrgPatch
		if e := decode(&in); e != nil {
			return Result{}, e
		}
		return s.PatchOrg(ctx, realm, token, parts[1], match, requestID, in, raw)
	case n == 2 && method == "DELETE":
		return keyed(func() (Result, error) {
			if e := empty(); e != nil {
				return Result{}, e
			}
			return s.DeleteOrg(ctx, realm, token, parts[1], match, key, requestID, raw)
		})
	case n == 3 && parts[2] == "members" && method == "POST":
		return keyed(func() (Result, error) {
			var in MemberInput
			if e := decode(&in); e != nil {
				return Result{}, e
			}
			return s.MemberWrite(ctx, realm, token, parts[1], "add", "", key, requestID, in, raw)
		})
	case n == 4 && parts[2] == "members" && method == "PUT":
		return keyed(func() (Result, error) {
			var in struct {
				Role string `json:"role"`
			}
			if e := decode(&in); e != nil {
				return Result{}, e
			}
			return s.MemberWrite(ctx, realm, token, parts[1], "role", parts[3], key, requestID, MemberInput{Role: in.Role}, raw)
		})
	case n == 4 && parts[2] == "members" && method == "DELETE":
		return keyed(func() (Result, error) {
			if e := empty(); e != nil {
				return Result{}, e
			}
			return s.MemberWrite(ctx, realm, token, parts[1], "remove", parts[3], key, requestID, MemberInput{}, raw)
		})
	case n == 3 && parts[2] == "users" && method == "POST":
		return keyed(func() (Result, error) {
			var in OrgUserInput
			if e := decode(&in); e != nil {
				return Result{}, e
			}
			return s.CreateOrgUser(ctx, realm, token, parts[1], key, requestID, in, raw)
		})
	case n == 3 && parts[2] == "groups" && method == "POST":
		return keyed(func() (Result, error) {
			var in GroupInput
			if e := decode(&in); e != nil {
				return Result{}, e
			}
			return s.OrgGroupWrite(ctx, realm, token, parts[1], "", "create", "", match, key, requestID, in, raw)
		})
	case n == 4 && parts[2] == "groups" && method == "PATCH":
		var in struct {
			Name *string `json:"name"`
		}
		if e := decode(&in); e != nil {
			return Result{}, e
		}
		if in.Name == nil {
			return Result{}, fault(400, "INVALID_REQUEST")
		}
		return s.OrgGroupWrite(ctx, realm, token, parts[1], parts[3], "patch", "", match, "", requestID, GroupInput{Name: *in.Name}, raw)
	case n == 4 && parts[2] == "groups" && method == "DELETE":
		return keyed(func() (Result, error) {
			if e := empty(); e != nil {
				return Result{}, e
			}
			return s.OrgGroupWrite(ctx, realm, token, parts[1], parts[3], "delete", "", match, key, requestID, GroupInput{}, raw)
		})
	case n == 5 && parts[2] == "groups" && parts[4] == "members" && method == "POST":
		return keyed(func() (Result, error) {
			var in struct {
				UserID string `json:"userId"`
			}
			if e := decode(&in); e != nil {
				return Result{}, e
			}
			return s.OrgGroupWrite(ctx, realm, token, parts[1], parts[3], "add", in.UserID, match, key, requestID, GroupInput{}, raw)
		})
	case n == 6 && parts[2] == "groups" && parts[4] == "members" && method == "DELETE":
		return keyed(func() (Result, error) {
			if e := empty(); e != nil {
				return Result{}, e
			}
			return s.OrgGroupWrite(ctx, realm, token, parts[1], parts[3], "remove", parts[5], match, key, requestID, GroupInput{}, raw)
		})
	}
	return Result{}, fault(404, "IDENTITY_NOT_FOUND")
}
