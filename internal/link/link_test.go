package link

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsure_CreatesLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link")
	if err := Ensure(target, linkPath); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	fi, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 && fi.Mode()&os.ModeIrregular == 0 {
		t.Errorf("expected symlink/junction, mode=%v", fi.Mode())
	}
}

func TestEnsure_IdempotentWhenAlreadyCorrect(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link")
	if err := Ensure(target, linkPath); err != nil {
		t.Fatal(err)
	}
	// Second call should no-op (no error).
	if err := Ensure(target, linkPath); err != nil {
		t.Fatalf("idempotent Ensure failed: %v", err)
	}
}

func TestEnsure_RefusesToClobberRegularDir(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a regular dir at the link path.
	linkPath := filepath.Join(dir, "link")
	if err := os.MkdirAll(linkPath, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Ensure(target, linkPath)
	var clob *ErrWouldClobber
	if !errors.As(err, &clob) {
		t.Errorf("expected ErrWouldClobber, got %v", err)
	}
}

func TestRemove_DropsLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link")
	if err := Ensure(target, linkPath); err != nil {
		t.Fatal(err)
	}
	if err := Remove(linkPath); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Errorf("expected link removed, err=%v", err)
	}
}

func TestRemove_RefusesRegularDir(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.MkdirAll(regular, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Remove(regular); err == nil {
		t.Error("expected Remove to refuse a regular directory")
	}
}

func TestIsManagedLink_TrueForOurLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(dir, "link")
	if err := Ensure(target, linkPath); err != nil {
		t.Fatal(err)
	}
	ok, err := IsManagedLink(linkPath, target)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("expected IsManagedLink to return true")
	}
}

func TestIsManagedLink_FalseForRegularDir(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.MkdirAll(regular, 0o755); err != nil {
		t.Fatal(err)
	}
	ok, _ := IsManagedLink(regular, "anything")
	if ok {
		t.Errorf("regular dir reported as managed link")
	}
}

func TestIsManagedLink_MissingReturnsError(t *testing.T) {
	if _, err := IsManagedLink(filepath.Join(t.TempDir(), "nope"), "x"); err == nil {
		t.Errorf("expected error for missing path")
	}
}

func TestErrWouldClobber_Error(t *testing.T) {
	e := &ErrWouldClobber{Path: "/tmp/x"}
	if e.Error() == "" {
		t.Error("expected non-empty Error()")
	}
}
