package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareAssignsServerIDsAndKeepsSafeCorrelation(t *testing.T) {
	var seen IDs
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = From(r.Context()) }))
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Request-ID", "client-chosen-id")
	r.Header.Set("X-Correlation-ID", "action-123456")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if seen.Request == "client-chosen-id" || seen.Request == "" || w.Header().Get("X-Request-ID") != seen.Request {
		t.Fatal("request ID must be server generated and returned", seen)
	}
	if seen.Correlation != "action-123456" {
		t.Fatal("safe correlation ID dropped", seen)
	}
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Correlation-ID", "bad id\nwith newline")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if seen.Correlation != seen.Request {
		t.Fatal("unsafe correlation ID accepted", seen)
	}
	if a, b := From(context.Background()), From(context.Background()); a.Request == "" || a.Request == b.Request {
		t.Fatal("fallback IDs must be fresh")
	}
}
