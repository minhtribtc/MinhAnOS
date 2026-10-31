package drive

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// OpenZFS before 2.2 answers EINVAL to RENAME_NOREPLACE: the fallback must still never replace a target.
func TestRenameNoReplaceFallback(t *testing.T) {
	old := renameat2
	renameat2 = func(int, string, int, string, uint) error { return unix.EINVAL }
	defer func() { renameat2 = old }()
	dir := t.TempDir()
	d, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	fd := int(d.Fd())
	os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0600)
	os.WriteFile(filepath.Join(dir, "b"), []byte("b"), 0600)
	if err = renameNoReplace(fd, "a", fd, "b"); !errors.Is(err, unix.EEXIST) {
		t.Fatal("file replaced", err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "b")); string(got) != "b" {
		t.Fatal("target changed", string(got))
	}
	if err = renameNoReplace(fd, "a", fd, "c"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Fatal("source kept after a file move", err)
	}
	os.Mkdir(filepath.Join(dir, "x"), 0700)
	os.Mkdir(filepath.Join(dir, "y"), 0700)
	if err = renameNoReplace(fd, "x", fd, "y"); !errors.Is(err, unix.EEXIST) {
		t.Fatal("directory replaced", err)
	}
	if err = renameNoReplace(fd, "x", fd, "z"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(dir, "z")); err != nil || !st.IsDir() {
		t.Fatal("directory move", err)
	}
}
