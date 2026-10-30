package platform

import (
	"net/http/httptest"
	"testing"
)

func TestDriveProxyRequiresDesktopSession(t *testing.T) {
	t.Setenv("DRIVE_INTERNAL_URL", "http://127.0.0.1:12345")
	a := &App{}
	r := httptest.NewRequest("GET", "http://desktop.test/api/drive/files/list", nil)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatal("unauthenticated gateway access", w.Code)
	}
}
