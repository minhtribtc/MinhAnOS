package identity

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotReady = errors.New("identity UUID backfill is incomplete")

// User contains public metadata only. No storage ID, hash, token or credential secret.
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
}
type Repository struct{ DB *pgxpool.Pool }
type BackfillStatus struct {
	Users    int64 `json:"users"`
	Pending  int64 `json:"pending"`
	Complete bool  `json:"complete"`
}

func (r Repository) Status(ctx context.Context) (BackfillStatus, error) {
	var s BackfillStatus
	e := r.DB.QueryRow(ctx, "SELECT count(*),count(*) FILTER(WHERE public_id IS NULL OR password_credential_id IS NULL) FROM users").Scan(&s.Users, &s.Pending)
	s.Complete = s.Pending == 0
	return s, e
}

// BackfillBatch is explicitly operator-driven, bounded, restartable, and safe for
// concurrent workers. The remaining NULLs are the checkpoint; assigned IDs never change.
func (r Repository) BackfillBatch(ctx context.Context, limit int) (int64, error) {
	if limit < 1 || limit > 1000 {
		return 0, errors.New("batch size must be 1..1000")
	}
	tag, e := r.DB.Exec(ctx, `WITH pending AS (SELECT id FROM users WHERE public_id IS NULL OR password_credential_id IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE users u SET public_id=coalesce(u.public_id,gen_random_uuid()),password_credential_id=coalesce(u.password_credential_id,gen_random_uuid()) FROM pending WHERE u.id=pending.id`, limit)
	return tag.RowsAffected(), e
}

// PublicUser is a data mapper, not authorization. Callers must authenticate and
// authorize the resource before using it; it has no public HTTP endpoint in Phase 1.
func (r Repository) PublicUser(ctx context.Context, legacyID string) (User, error) {
	var u User
	var id pgtype.Text
	e := r.DB.QueryRow(ctx, "SELECT public_id::text,username,display_name,status,version FROM users WHERE id=$1", legacyID).Scan(&id, &u.Username, &u.DisplayName, &u.Status, &u.Version)
	if e != nil {
		return u, e
	}
	if !id.Valid {
		return User{}, ErrNotReady
	}
	u.ID = id.String
	return u, nil
}
func (r Repository) LegacyID(ctx context.Context, publicID string) (string, error) {
	var id string
	e := r.DB.QueryRow(ctx, "SELECT id FROM users WHERE public_id::text=$1", publicID).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return "", e
	}
	return id, e
}
