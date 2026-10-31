package drive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"minhanos.local/platform/internal/storagecore"
)

// memLedger is an in-memory QuotaLedger with one home limit.
type memLedger struct {
	mu                          sync.Mutex
	limit, used, reserved       int64
	commits, releases, measured int
	needMeasure                 bool
}

func (m *memLedger) Reserve(_ context.Context, home string, n int64) (storagecore.Reservation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.limit > 0 && m.used+m.reserved+n > m.limit {
		return storagecore.Reservation{}, storagecore.QuotaExceeded{Scope: "home"}
	}
	m.reserved += n
	return storagecore.Reservation{ID: "r", Home: home, Bytes: n}, nil
}
func (m *memLedger) Commit(_ context.Context, r storagecore.Reservation, actual int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reserved -= r.Bytes
	m.used += actual
	m.commits++
	return nil
}
func (m *memLedger) Release(_ context.Context, r storagecore.Reservation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reserved -= r.Bytes
	m.releases++
	return nil
}
func (m *memLedger) NeedsMeasure(context.Context, string) (bool, error) { return m.needMeasure, nil }
func (m *memLedger) PendingMeasures(context.Context, int) ([]string, error) {
	if m.needMeasure {
		return []string{strings.Repeat("a", 64)}, nil
	}
	return nil, nil
}
func (m *memLedger) SetMeasured(_ context.Context, _ string, n int64) error {
	m.used, m.needMeasure = n, false
	m.measured++
	return nil
}
func (m *memLedger) HomeUsage(context.Context, string) (storagecore.Usage, error) {
	l := m.limit
	return storagecore.Usage{LimitBytes: &l, UsedBytes: m.used, Measured: !m.needMeasure}, nil
}

func quotaRequest(t *testing.T, s *Server, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "http://drive.test"+target, strings.NewReader(body))
	r.AddCookie(&http.Cookie{Name: CookieName, Value: "valid"})
	r.Header.Set("X-CSRF-Token", "csrf")
	r.Header.Set("Origin", "http://drive.test")
	if method == "POST" {
		r.Header.Set("Idempotency-Key", "test-"+strconv.FormatInt(time.Now().UnixNano(), 36)+"-key")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestPersonalWritesUseSoftQuota(t *testing.T) {
	ledger := &memLedger{limit: 100, needMeasure: true}
	s := &Server{Identity: fakeIdentity{role: "user"}, DataDir: t.TempDir(), StaticDir: t.TempDir(), Quota: ledger}
	if w := quotaRequest(t, s, "GET", "/api/roots", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"measured":false`) || ledger.measured != 0 {
		t.Fatal("roots must report quota without scanning on the request path", w.Code, w.Body.String(), ledger.measured)
	}
	if e := s.measurePending(context.Background()); e != nil || ledger.measured != 1 || ledger.needMeasure {
		t.Fatal("background measurement", e, ledger.measured)
	}
	if w := quotaRequest(t, s, "POST", "/api/files/upload?root=personal&path=&name=a.txt", strings.Repeat("x", 60)); w.Code != 201 {
		t.Fatal("upload under quota", w.Code, w.Body.String())
	}
	if ledger.used != 60 || ledger.reserved != 0 || ledger.commits != 1 {
		t.Fatalf("commit %+v", ledger)
	}
	w := quotaRequest(t, s, "POST", "/api/files/upload?root=personal&path=&name=b.txt", strings.Repeat("y", 60))
	if w.Code != 507 || !strings.Contains(w.Body.String(), "QUOTA_EXCEEDED") {
		t.Fatal("over quota upload", w.Code, w.Body.String())
	}
	// A failed write (name collision) releases its reservation.
	if w = quotaRequest(t, s, "POST", "/api/files/upload?root=personal&path=&name=a.txt", "zz"); w.Code == 201 {
		t.Fatal("collision accepted")
	}
	if ledger.reserved != 0 || ledger.releases != 1 || ledger.used != 60 {
		t.Fatalf("release %+v", ledger)
	}
	// Operations that add no data never reserve.
	if w = quotaRequest(t, s, "POST", "/api/files/mkdir?root=personal&path=", `{"name":"docs"}`); w.Code != 200 || ledger.commits != 1 {
		t.Fatal("mkdir", w.Code, w.Body.String(), ledger.commits)
	}
}
