package platform

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// User data ownership (app standard row 17, contracts/data/desktop.data.json). A user downloads their own
// Desktop data (layout, pinned apps and notes) from GET /api/export. Account Center deletes are final
// (IDENTITY_USER_DELETED), so the Desktop removes the data of deleted accounts on its next hourly pass.
const removalInterval = time.Hour

func (a *App) exportData(w http.ResponseWriter, r *http.Request) {
	prefs, err := storedPreferences(r.Context(), a.db, identity(r).User.ID, false)
	if err != nil {
		internalError(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="minhanos-desktop.json"`)
	writeJSON(w, 200, map[string]any{"app": "com.minhanos.desktop", "exportVersion": "1", "exportedAt": time.Now().UTC(), "preferences": prefs})
}

// RemoveDeletedUsers deletes the Desktop rows of accounts deleted in Account Center: preferences (with the
// notes) and the legacy audit rows that still name them.
func RemoveDeletedUsers(ctx context.Context, db *pgxpool.Pool) (int64, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, "DELETE FROM preferences WHERE user_id IN (SELECT id FROM users WHERE status='deleted')")
	if err != nil {
		return 0, err
	}
	audit, err := tx.Exec(ctx, "DELETE FROM audit WHERE user_id IN (SELECT id FROM users WHERE status='deleted')")
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected() + audit.RowsAffected(), tx.Commit(ctx)
}

func (a *App) runRemovals(ctx context.Context) {
	for {
		if n, err := RemoveDeletedUsers(ctx, a.db); err != nil && ctx.Err() == nil {
			slog.Error("desktop.user_data.removal_failed", "error", err.Error())
		} else if n > 0 {
			slog.Info("desktop.user_data.removed", "rows", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(removalInterval):
		}
	}
}
