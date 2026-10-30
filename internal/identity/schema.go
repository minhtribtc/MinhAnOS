// Package identity owns additive identity schema and mapping. Authentication/policy
// adapters remain legacy until Phase 2; schema presence never grants access.
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

func Module() migrate.Module {
	return migrate.Module{ID: "identity", Steps: []migrate.Step{{ID: 1, Name: "adopt_legacy_users", SQL: adoptSQL}, {ID: 2, Name: "expand_public_identity", SQL: expandSQL}, {ID: 3, Name: "empty_identity_relations", SQL: relationsSQL}}}
}

// ApplySchema must be called after the legacy bootstrap transaction commits.
func ApplySchema(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, Module())
}
