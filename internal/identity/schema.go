// Package identity owns additive identity schema and mapping. Authentication/policy
// adapters share AuthCore; schema presence never grants authorization.
package identity

import (
	"context"
	_ "embed"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
)

//go:embed migrations/001_adopt.sql
var adoptSQL string

//go:embed migrations/002_expand.sql
var expandSQL string

//go:embed migrations/003_relations.sql
var relationsSQL string

//go:embed migrations/004_auth_core.sql
var authCoreSQL string

//go:embed migrations/005_policy.sql
var policySQL string

//go:embed migrations/006_auth_audit.sql
var authAuditSQL string

//go:embed migrations/007_security_log.sql
var securityLogSQL string

func Module() migrate.Module {
	return migrate.Module{ID: "identity", Steps: []migrate.Step{{ID: 1, Name: "adopt_legacy_users", SQL: adoptSQL}, {ID: 2, Name: "expand_public_identity", SQL: expandSQL}, {ID: 3, Name: "empty_identity_relations", SQL: relationsSQL}, {ID: 4, Name: "shared_auth_core", SQL: authCoreSQL}, {ID: 5, Name: "identity_policy", SQL: policySQL}, {ID: 6, Name: "auth_audit", SQL: authAuditSQL}, {ID: 7, Name: "security_log", SQL: securityLogSQL}}}
}

// ApplySchema must be called after the legacy bootstrap transaction commits.
func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}
