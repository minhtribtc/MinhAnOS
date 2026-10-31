// Package requestid gives every HTTP request a server-generated ID for audit and logs.
// A client-supplied X-Correlation-ID (safe characters only) links several requests of one
// user action; the request ID itself is never taken from the client.
package requestid

import (
	"context"
	"net/http"
	"regexp"

	"minhanos.local/platform/internal/authn"
)

type key struct{}

type IDs struct{ Request, Correlation string }

var safe = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids := IDs{Request: authn.Token()}
		ids.Correlation = ids.Request
		if c := r.Header.Get("X-Correlation-ID"); safe.MatchString(c) {
			ids.Correlation = c
		}
		w.Header().Set("X-Request-ID", ids.Request)
		w.Header().Set("X-Correlation-ID", ids.Correlation)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, ids)))
	})
}

// With attaches IDs to a context outside HTTP (a background job names its own).
func With(ctx context.Context, ids IDs) context.Context { return context.WithValue(ctx, key{}, ids) }

// From returns the IDs of the current request; outside HTTP (CLI, tests) it creates fresh ones.
func From(ctx context.Context) IDs {
	if ids, ok := ctx.Value(key{}).(IDs); ok {
		return ids
	}
	id := authn.Token()
	return IDs{id, id}
}
