package drive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedEngineScopes(t *testing.T) {
	home := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(home, "files", "Docs", "Sub"), 0755))
	must(os.MkdirAll(filepath.Join(home, "trash"), 0755))
	must(os.WriteFile(filepath.Join(home, "files", "Docs", "a.txt"), []byte("alpha"), 0644))
	must(os.WriteFile(filepath.Join(home, "files", "Docs", "Sub", "b.txt"), []byte("beta"), 0644))
	must(os.WriteFile(filepath.Join(home, "files", "secret.txt"), []byte("secret"), 0644))
	must(os.WriteFile(filepath.Join(home, "files", "note.txt"), []byte("note"), 0644))

	t.Run("folder share", func(t *testing.T) {
		e, root, err := sharedEngine(home, "Docs", true)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { root.Close() })
		entries, err := e.List(".")
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 2 || entries[0].Name != "Sub" || entries[1].Name != "a.txt" {
			t.Fatalf("unexpected list: %+v", entries)
		}
		text, _, err := e.Text("a.txt")
		if err != nil || text != "alpha" {
			t.Fatalf("Text(a.txt) = %q, %v", text, err)
		}
		if _, err = e.logical("../secret.txt"); err == nil {
			t.Fatal("logical(../secret.txt) succeeded")
		}
		item, err := e.Trash("a.txt")
		if err != nil {
			t.Fatal(err)
		}
		if item.Path != "Docs/a.txt" {
			t.Fatalf("trash path = %q", item.Path)
		}
		_, err = e.TrashList()
		var fe *FileError
		if !errors.As(err, &fe) || fe.Code != "FILES_SHARED_NO_TRASH" {
			t.Fatalf("TrashList error = %v", err)
		}
		found, _, err := e.Search(".", "b.txt")
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].Path != "Sub/b.txt" {
			t.Fatalf("Search = %+v", found)
		}
	})

	t.Run("file share", func(t *testing.T) {
		e, root, err := sharedEngine(home, "note.txt", false)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { root.Close() })
		entries, err := e.List(".")
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name != "note.txt" {
			t.Fatalf("unexpected list: %+v", entries)
		}
		text, _, err := e.Text("note.txt")
		if err != nil || text != "note" {
			t.Fatalf("Text(note.txt) = %q, %v", text, err)
		}
		_, _, err = e.Text("secret.txt")
		var fe *FileError
		if !errors.As(err, &fe) || fe.Code != "FILES_NOT_FOUND" {
			t.Fatalf("Text(secret.txt) error = %v", err)
		}
		found, _, err := e.Search(".", "t")
		if err != nil {
			t.Fatal(err)
		}
		if len(found) != 1 || found[0].Path != "note.txt" {
			t.Fatalf("Search = %+v", found)
		}
	})

	t.Run("invalid scopes", func(t *testing.T) {
		if _, root, err := sharedEngine(home, "missing", true); err == nil {
			if root != nil {
				root.Close()
			}
			t.Fatal("missing folder share succeeded")
		}
		if _, root, err := sharedEngine(home, "note.txt", true); err == nil {
			if root != nil {
				root.Close()
			}
			t.Fatal("folder share of file succeeded")
		}
	})
}
