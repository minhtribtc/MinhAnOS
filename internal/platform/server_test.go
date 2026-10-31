package platform

import (
	"net/http/httptest"
	"testing"
)

func TestPasswordVerification(t *testing.T) {
	hash, err := hashPassword("correct-password-123")
	if err != nil {
		t.Fatal(err)
	}
	other, err := hashPassword("correct-password-123")
	if err != nil {
		t.Fatal(err)
	}
	if hash == other {
		t.Fatal("password salt reused")
	}
	if !verifyPassword("correct-password-123", hash) || verifyPassword("wrong-password", hash) || verifyPassword("x", "malformed") {
		t.Fatal("password verification failed")
	}
}
func TestCrossOriginRejected(t *testing.T) {
	r := httptest.NewRequest("POST", "http://10.1.1.20:8081/api/logout", nil)
	r.Header.Set("Origin", "http://attacker.example")
	if sameOrigin(r) {
		t.Fatal("cross-origin accepted")
	}
	r.Header.Set("Origin", "http://10.1.1.20:8081")
	if !sameOrigin(r) {
		t.Fatal("same-origin rejected")
	}
	r.Header.Del("Origin")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	if sameOrigin(r) {
		t.Fatal("cross-site accepted")
	}
}
func TestUntrustedWindowState(t *testing.T) {
	p := defaultPreferences()
	if !validPreferences(p) {
		t.Fatal("defaults invalid")
	}
	window := Window{ID: "files", X: 250, Y: 95, Width: 1000, Height: 650}
	p.Windows = []Window{window, window}
	if validPreferences(p) {
		t.Fatal("duplicate window accepted")
	}
	p.Windows = []Window{window}
	p.Windows[0].ID = "../../shell"
	if validPreferences(p) {
		t.Fatal("unknown app accepted")
	}
	p.Windows = []Window{window}
	p.Windows[0].Width = -2
	if validPreferences(p) {
		t.Fatal("invalid geometry accepted")
	}
}

func TestWorkspaceModes(t *testing.T) {
	p := defaultPreferences()
	if p.Mode != "focus" {
		t.Fatal("first workspace should use focus mode")
	}
	p.Mode = "windows"
	if !validPreferences(p) {
		t.Fatal("window mode rejected")
	}
	p.Mode = "arbitrary-layout"
	if validPreferences(p) {
		t.Fatal("unknown workspace mode accepted")
	}
}
func TestMissingSessionRejected(t *testing.T) {
	a := &App{}
	handler := a.auth(nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/preferences", nil)
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("wanted 401, got %d", w.Code)
	}
}

func TestRouteRegistration(t *testing.T) {
	a := &App{}
	handler := a.Handler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/unknown", nil)
	handler.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatalf("unknown API: expected 404, got %d", w.Code)
	}
}

// Saved layouts from before 2026-10-31 keep working: removed built-in apps drop out, "notes" is renamed.
func TestKnownAppsMigratesLegacyLayout(t *testing.T) {
	p := knownApps(Preferences{Windows: []Window{{ID: "overview"}, {ID: "notes"}, {ID: "files"}}, Pinned: []string{"apps", "notes", "files", "activity"}})
	if len(p.Windows) != 2 || p.Windows[0].ID != "com.minhanos.notes" || len(p.Pinned) != 2 || p.Pinned[0] != "com.minhanos.notes" || p.Pinned[1] != "files" {
		t.Fatal("legacy layout", p)
	}
}
