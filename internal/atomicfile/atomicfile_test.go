package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func runtimeIsWindows() bool { return runtime.GOOS == "windows" }

func TestWriteFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q want %q", got, "hello")
	}
}

func TestWriteFile_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(p, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "new" {
		t.Fatalf("got %q want %q", got, "new")
	}
}

func TestWriteFile_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nested", "deep", "x.txt")
	if err := WriteFile(p, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestWriteFile_ParentIsAFile(t *testing.T) {
	dir := t.TempDir()
	// Make a regular file, then try to write inside it as if it were a dir.
	notADir := filepath.Join(dir, "blocker")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(notADir, "child", "f.txt")
	if err := WriteFile(target, []byte("hi"), 0o644); err == nil {
		t.Error("expected error: parent of target is a regular file")
	}
}

func TestWriteFile_PreservesPermissionsArg(t *testing.T) {
	if runtimeIsWindows() {
		t.Skip("file modes don't map cleanly on Windows")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := WriteFile(p, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", fi.Mode().Perm())
	}
}

func TestWriteFile_NoTempLeftBehindOnSuccess(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := WriteFile(p, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "x.txt" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("unexpected dir contents: %v", names)
	}
}
