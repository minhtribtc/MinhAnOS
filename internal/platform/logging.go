package platform

import (
	"context"
	"log/slog"

	"minhanos.local/platform/internal/requestid"
)

// Operational logs are structured (app standard row 12): one event name, the request and correlation
// IDs, and identifiers only — no usernames, file paths or file names.
func logError(ctx context.Context, event string, err error, attrs ...any) {
	ids := requestid.From(ctx)
	slog.ErrorContext(ctx, event, append([]any{"request_id", ids.Request, "correlation_id", ids.Correlation, "error", err.Error()}, attrs...)...)
}

func logInfo(ctx context.Context, event string, attrs ...any) {
	ids := requestid.From(ctx)
	slog.InfoContext(ctx, event, append([]any{"request_id", ids.Request, "correlation_id", ids.Correlation}, attrs...)...)
}
