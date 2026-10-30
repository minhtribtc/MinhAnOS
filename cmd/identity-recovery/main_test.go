package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateInputs(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root-owned Linux private input test")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if e := os.WriteFile(path, []byte("a-secret-value\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if v, e := privateRead(path); e != nil || v != "a-secret-value" {
		t.Fatal("private file rejected", e)
	}
	if e := os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := privateRead(path); e == nil {
		t.Fatal("public file accepted")
	}
	os.Chmod(path, 0600)
	link := filepath.Join(dir, "link")
	if e := os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e := privateRead(link); e == nil {
		t.Fatal("symlink accepted")
	}
	if _, e := privateRead(dir); e == nil {
		t.Fatal("directory accepted")
	}
	os.WriteFile(path, []byte(strings.Repeat("x", 4097)), 0600)
	if _, e := privateRead(path); e == nil {
		t.Fatal("oversized file accepted")
	}
}
