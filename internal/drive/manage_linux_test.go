package drive

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFile(t *testing.T, e *Engine, p, content string) {
	t.Helper()
	full := filepath.Join(e.Root.Name(), "files", filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestSearchFindsNestedCaseInsensitiveAndRejectsEmpty(t *testing.T) {
	e := testEngine(t)
	writeFile(t, e, "docs/Work/Report.TXT", "x")
	writeFile(t, e, "docs/notes.txt", "y")
	if err := os.MkdirAll(filepath.Join(e.Root.Name(), "files", "docs", "Reports"), 0700); err != nil {
		t.Fatal(err)
	}
	got, truncated, err := e.Search("", "report")
	if err != nil || truncated || len(got) != 2 {
		t.Fatal(got, truncated, err)
	}
	if _, _, err = e.Search("", "  "); err == nil {
		t.Fatal("empty query accepted")
	}
}

func TestRecentReturnsOnlyRegularFilesNewestFirst(t *testing.T) {
	e := testEngine(t)
	writeFile(t, e, "a.txt", "a")
	writeFile(t, e, "sub/b.txt", "b")
	if err := os.MkdirAll(filepath.Join(e.Root.Name(), "files", "sub", "folder"), 0700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	mid := time.Now().Add(-time.Minute)
	for p, m := range map[string]time.Time{"a.txt": old, "sub/b.txt": mid} {
		if err := os.Chtimes(filepath.Join(e.Root.Name(), "files", filepath.FromSlash(p)), m, m); err != nil {
			t.Fatal(err)
		}
	}
	got, truncated, err := e.Recent("")
	if err != nil || truncated || len(got) != 2 || got[0].Name != "b.txt" || got[1].Name != "a.txt" {
		t.Fatal(got, truncated, err)
	}
}

func TestArchiveCheckAndArchive(t *testing.T) {
	e := testEngine(t)
	writeFile(t, e, "album/a.txt", "inside")
	writeFile(t, e, "b.txt", "outside")
	if _, err := e.ArchiveCheck(nil); err == nil {
		t.Fatal("nil items accepted")
	}
	if _, err := e.ArchiveCheck([]string{"."}); err == nil {
		t.Fatal("root archive accepted")
	}
	if _, err := e.ArchiveCheck([]string{"missing"}); err == nil {
		t.Fatal("missing item accepted")
	}
	var buf bytes.Buffer
	if err := e.Archive(&buf, []string{"album", "b.txt"}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"album/": "", "album/a.txt": "inside", "b.txt": "outside"}
	if len(zr.File) != len(want) {
		t.Fatalf("zip entries: %v", zr.File)
	}
	for _, f := range zr.File {
		content, ok := want[f.Name]
		if !ok {
			t.Fatalf("unexpected entry %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		if string(data) != content {
			t.Fatalf("%s: got %q", f.Name, data)
		}
	}
}

func TestEmptyTrashSingleAllAndInvalid(t *testing.T) {
	e := testEngine(t)
	writeFile(t, e, "a.txt", "first")
	first, err := e.Trash("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, e, "a.txt", "new a")
	second, err := e.Trash("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	freed, err := e.EmptyTrash(first.ID)
	if err != nil || freed < 5 { // content plus the trash metadata
		t.Fatal(freed, err)
	}
	items, err := e.TrashList()
	if err != nil || len(items) != 1 || items[0].ID != second.ID {
		t.Fatal(items, err)
	}
	if _, err = e.EmptyTrash(""); err != nil {
		t.Fatal(err)
	}
	if items, _ = e.TrashList(); len(items) != 0 {
		t.Fatal("trash not empty", items)
	}
	if _, err = e.EmptyTrash("../x"); err == nil {
		t.Fatal("traversal id accepted")
	}
}

func TestServeEngineArchiveHeaders(t *testing.T) {
	e := testEngine(t)
	writeFile(t, e, "album/a.txt", "x")
	r := httptest.NewRequest("GET", "http://drive/api/files/archive?root=personal&path=&item=album", nil)
	w := httptest.NewRecorder()
	if err := ServeEngine(w, r, e); err != nil {
		t.Fatal(err)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatal(ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "album.zip") {
		t.Fatal(cd)
	}
}

func TestArchiveKeepsItemsWithTheSameName(t *testing.T) {
	e := testEngine(t)
	writeFile(t, e, "album/a.txt", "one")
	writeFile(t, e, "other/a.txt", "two")
	items, err := e.ArchiveCheck([]string{"album/a.txt", "other/a.txt", "album/a.txt"})
	if err != nil || len(items) != 2 {
		t.Fatal(items, err)
	}
	var buf bytes.Buffer
	if err = e.Archive(&buf, items); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil || len(zr.File) != 2 || zr.File[0].Name != "a.txt" || zr.File[1].Name != "a (2).txt" {
		t.Fatal(zr, err)
	}
}
