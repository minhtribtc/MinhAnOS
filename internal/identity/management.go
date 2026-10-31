package identity

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var policyKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)*$`)

type Group struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Version int64  `json:"version"`
}
type GroupInput struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}
type RoleInput struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}
type BindingInput struct {
	RoleID  string `json:"roleId"`
	Subject struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	} `json:"subject"`
	Scope struct {
		Kind string  `json:"kind"`
		ID   *string `json:"id"`
	} `json:"scope"`
}

func managementActor(a Actor) error {
	if a.Role != "admin" {
		return fault(403, "IDENTITY_PROTECTED_POLICY")
	}
	return nil
}
func validPolicyName(key, name string) bool {
	return len(key) <= 100 && policyKeyPattern.MatchString(key) && len([]rune(strings.TrimSpace(name))) > 0 && len([]rune(name)) <= 100
}
func readGroup(ctx context.Context, tx pgx.Tx, id string) (Group, error) {
	var g Group
	if !uuidPattern.MatchString(id) {
		return g, fault(404, "IDENTITY_NOT_FOUND")
	}
	e := tx.QueryRow(ctx, "SELECT id::text,key,name,kind,version FROM identity_groups WHERE id=$1", id).Scan(&g.ID, &g.Key, &g.Name, &g.Kind, &g.Version)
	if errors.Is(e, pgx.ErrNoRows) {
		return g, fault(404, "IDENTITY_NOT_FOUND")
	}
	return g, e
}
func (s *Service) GroupWrite(ctx context.Context, realm Realm, token, id, operation, member, match, key, requestID string, input GroupInput, raw []byte) (Result, error) {
	return s.groupWrite(ctx, realm, token, "", id, operation, member, match, key, requestID, input, raw)
}

// OrgGroupWrite manages a group of one organization (its admins or Root; ADR 0008). Only members can join.
func (s *Service) OrgGroupWrite(ctx context.Context, realm Realm, token, org, id, operation, member, match, key, requestID string, input GroupInput, raw []byte) (Result, error) {
	if !uuidPattern.MatchString(org) {
		return Result{}, fault(404, "ORG_NOT_FOUND")
	}
	return s.groupWrite(ctx, realm, token, org, id, operation, member, match, key, requestID, input, raw)
}

// groupWrite: org "" is the platform route (System Admin, any group); new groups land in the default organization.
func (s *Service) groupWrite(ctx context.Context, realm Realm, token, org, id, operation, member, match, key, requestID string, input GroupInput, raw []byte) (Result, error) {
	base, permission := "/api/v1/identity/groups", "identity.groups.write"
	if org != "" {
		base, permission = "/api/v1/identity/organizations/"+org+"/groups", ""
	}
	route := map[string]string{"create": "POST " + base, "patch": "PATCH " + base + "/" + id, "add": "POST " + base + "/" + id + "/members", "remove": "DELETE " + base + "/" + id + "/members/" + member, "delete": "DELETE " + base + "/" + id}[operation]
	return s.mutate(ctx, realm, token, permission, requestID, key, route, append([]byte(match+"\x00"), raw...), true, func(tx pgx.Tx, a Actor) (Result, error) {
		if org == "" {
			if e := managementActor(a); e != nil {
				return Result{}, e
			}
		} else if _, e := needRole(ctx, tx, a, org, "root", "admin"); e != nil {
			return Result{}, e
		}
		owner := org
		if operation == "create" {
			input.Name = strings.TrimSpace(input.Name)
			if !validPolicyName(input.Key, input.Name) {
				return Result{}, fault(422, "IDENTITY_INVALID_GROUP")
			}
			var g Group
			e := tx.QueryRow(ctx, "INSERT INTO identity_groups(key,name,kind,organization_id) VALUES($1,$2,'user',coalesce(nullif($3,'')::uuid,(SELECT id FROM identity_organizations WHERE is_default))) ON CONFLICT DO NOTHING RETURNING id::text,key,name,kind,version,coalesce(organization_id::text,'')", input.Key, input.Name, org).Scan(&g.ID, &g.Key, &g.Name, &g.Kind, &g.Version, &owner)
			if errors.Is(e, pgx.ErrNoRows) {
				return Result{}, fault(409, "IDENTITY_KEY_EXISTS")
			}
			if e != nil {
				return Result{}, e
			}
			out := result(g, 201, g.Version)
			out.Org = owner
			return out, nil
		}
		var g Group
		var e error
		if org != "" {
			g, e = orgGroup(ctx, tx, org, id)
		} else {
			g, e = readGroup(ctx, tx, id)
			if e == nil {
				e = tx.QueryRow(ctx, "SELECT coalesce(organization_id::text,'') FROM identity_groups WHERE id=$1", id).Scan(&owner)
			}
		}
		if e != nil {
			return Result{}, e
		}
		if g.Kind != "user" {
			return Result{}, fault(403, "IDENTITY_PROTECTED_POLICY")
		}
		if e = requireVersion(g.Version, match); e != nil {
			return Result{}, e
		}
		withOrg := func(r Result) Result { r.Org = owner; return r }
		switch operation {
		case "delete":
			if _, e = tx.Exec(ctx, "DELETE FROM identity_groups WHERE id=$1", id); e != nil {
				return Result{}, e
			}
			return withOrg(result(map[string]bool{"accepted": true}, 200, 0)), nil
		case "patch":
			input.Name = strings.TrimSpace(input.Name)
			if len([]rune(input.Name)) < 1 || len([]rune(input.Name)) > 100 {
				return Result{}, fault(422, "IDENTITY_INVALID_GROUP")
			}
			g.Name = input.Name
		case "add", "remove":
			u, _, e := userRow(ctx, tx, member)
			if e != nil {
				return Result{}, e
			}
			if u.Status == "deleted" {
				return Result{}, fault(409, "IDENTITY_USER_DELETED")
			}
			if operation == "add" {
				// Organization groups only take members of the organization (trigger, identity SQL 012).
				_, e = savepoint(ctx, tx, "INSERT INTO identity_group_members(group_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING", id, member)
				if pgCode(e) == "P0001" {
					return Result{}, fault(409, "IDENTITY_NOT_MEMBER")
				}
			} else {
				_, e = tx.Exec(ctx, "DELETE FROM identity_group_members WHERE group_id=$1 AND user_id=$2", id, member)
			}
			if e != nil {
				return Result{}, e
			}
		default:
			return Result{}, fault(400, "INVALID_REQUEST")
		}
		if _, e = tx.Exec(ctx, "UPDATE identity_groups SET name=$2,version=version+1 WHERE id=$1", id, g.Name); e != nil {
			return Result{}, e
		}
		g.Version++
		if operation == "add" || operation == "remove" {
			return withOrg(result(map[string]bool{"accepted": true}, 200, g.Version)), nil
		}
		return withOrg(result(g, 200, g.Version)), nil
	})
}
func (s *Service) CreateRole(ctx context.Context, realm Realm, token, key, requestID string, input RoleInput, raw []byte) (Result, error) {
	return s.mutate(ctx, realm, token, "identity.roles.write", requestID, key, "POST /api/v1/identity/roles", raw, true, func(tx pgx.Tx, a Actor) (Result, error) {
		if e := managementActor(a); e != nil {
			return Result{}, e
		}
		input.Name = strings.TrimSpace(input.Name)
		if !validPolicyName(input.Key, input.Name) || input.Permissions == nil || len(input.Permissions) > 64 {
			return Result{}, fault(422, "IDENTITY_INVALID_ROLE")
		}
		seen := map[string]bool{}
		for _, p := range input.Permissions {
			if seen[p] {
				return Result{}, fault(422, "IDENTITY_INVALID_ROLE")
			}
			seen[p] = true
			// Policy authority is reserved to direct System Admin. A custom role cannot
			// mint further roles/groups/app policies or create a second admin path.
			if p == "identity.roles.write" || p == "identity.groups.write" || p == "identity.apps.write" {
				return Result{}, fault(403, "IDENTITY_PERMISSION_CEILING")
			}
			if e := authorize(ctx, tx, a, p); e != nil {
				return Result{}, e
			}
		}
		var id string
		e := tx.QueryRow(ctx, "INSERT INTO identity_roles(key,name) VALUES($1,$2) ON CONFLICT(key) DO NOTHING RETURNING id::text", input.Key, input.Name).Scan(&id)
		if errors.Is(e, pgx.ErrNoRows) {
			return Result{}, fault(409, "IDENTITY_KEY_EXISTS")
		}
		if e != nil {
			return Result{}, e
		}
		for _, p := range input.Permissions {
			if _, e = tx.Exec(ctx, "INSERT INTO identity_role_permissions(role_id,permission_key) VALUES($1,$2)", id, p); e != nil {
				return Result{}, e
			}
		}
		return result(map[string]any{"id": id, "key": input.Key, "name": input.Name, "protected": false, "permissions": input.Permissions, "version": 1}, 201, 1), nil
	})
}
func (s *Service) BindRole(ctx context.Context, realm Realm, token, id, match, key, requestID string, input BindingInput, raw []byte, remove bool) (Result, error) {
	route := "POST /api/v1/identity/role-bindings"
	if remove {
		route = "DELETE /api/v1/identity/role-bindings/" + id
	}
	return s.mutate(ctx, realm, token, "identity.roles.write", requestID, key, route, append([]byte(match+"\x00"), raw...), true, func(tx pgx.Tx, a Actor) (Result, error) {
		if e := managementActor(a); e != nil {
			return Result{}, e
		}
		if remove {
			if !uuidPattern.MatchString(id) {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			var version int64
			var roleKey string
			e := tx.QueryRow(ctx, "SELECT b.version,r.key FROM identity_role_bindings b JOIN identity_roles r ON r.id=b.role_id WHERE b.id=$1", id).Scan(&version, &roleKey)
			if errors.Is(e, pgx.ErrNoRows) {
				return Result{}, fault(404, "IDENTITY_NOT_FOUND")
			}
			if e != nil {
				return Result{}, e
			}
			if roleKey == "system-admin" {
				return Result{}, fault(403, "IDENTITY_PROTECTED_POLICY")
			}
			if e = requireVersion(version, match); e != nil {
				return Result{}, e
			}
			if _, e = tx.Exec(ctx, "DELETE FROM identity_role_bindings WHERE id=$1", id); e != nil {
				return Result{}, e
			}
			return result(map[string]bool{"accepted": true}, 200, 0), nil
		}
		if !uuidPattern.MatchString(input.RoleID) || !uuidPattern.MatchString(input.Subject.ID) {
			return Result{}, fault(422, "IDENTITY_INVALID_ROLE")
		}
		if input.Scope.Kind != "system" || input.Scope.ID != nil {
			return Result{}, fault(422, "IDENTITY_UNSUPPORTED_SCOPE")
		}
		var roleKey string
		var protected bool
		e := tx.QueryRow(ctx, "SELECT key,protected FROM identity_roles WHERE id=$1", input.RoleID).Scan(&roleKey, &protected)
		if errors.Is(e, pgx.ErrNoRows) {
			return Result{}, fault(404, "IDENTITY_NOT_FOUND")
		}
		if e != nil {
			return Result{}, e
		}
		if protected && roleKey != "user-admin" {
			return Result{}, fault(403, "IDENTITY_PROTECTED_POLICY")
		}
		var user, group any
		switch input.Subject.Kind {
		case "user":
			u, _, e := userRow(ctx, tx, input.Subject.ID)
			if e != nil {
				return Result{}, e
			}
			if u.Status == "deleted" {
				return Result{}, fault(409, "IDENTITY_USER_DELETED")
			}
			user = input.Subject.ID
		case "group":
			g, e := readGroup(ctx, tx, input.Subject.ID)
			if e != nil {
				return Result{}, e
			}
			if g.Kind != "user" {
				return Result{}, fault(403, "IDENTITY_PROTECTED_POLICY")
			}
			group = input.Subject.ID
		default:
			return Result{}, fault(422, "IDENTITY_INVALID_ROLE")
		}
		// Serialize uniqueness under the existing common policy lock without rewriting
		// immutable schema. Reject duplicates rather than issue parallel bindings.
		var exists bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM identity_role_bindings WHERE role_id=$1 AND user_id IS NOT DISTINCT FROM $2::uuid AND group_id IS NOT DISTINCT FROM $3::uuid AND scope=$4::jsonb)", input.RoleID, user, group, `{"kind":"system","id":null}`).Scan(&exists)
		if e != nil {
			return Result{}, e
		}
		if exists {
			return Result{}, fault(409, "IDENTITY_BINDING_EXISTS")
		}
		var binding string
		e = tx.QueryRow(ctx, `INSERT INTO identity_role_bindings(role_id,user_id,group_id,scope) VALUES($1,$2,$3,'{"kind":"system","id":null}') RETURNING id::text`, input.RoleID, user, group).Scan(&binding)
		if e != nil {
			return Result{}, e
		}
		return result(map[string]any{"id": binding, "roleId": input.RoleID, "subject": input.Subject, "scope": input.Scope, "version": 1}, 201, 1), nil
	})
}
