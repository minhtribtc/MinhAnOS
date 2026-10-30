package coreprofile

import (
	"context"
	_ "embed"
	"github.com/jackc/pgx/v5/pgxpool"
	"minhanos.local/platform/internal/migrate"
)

//go:embed migrations/001_profile.sql
var profileSchema string

func migrateProfile(ctx context.Context, db *pgxpool.Pool) error {
	return migrate.Apply(ctx, db, migrate.Module{ID: "profile", Steps: []migrate.Step{{ID: 1, Name: "profile_expand_adopt", SQL: profileSchema}}})
}
