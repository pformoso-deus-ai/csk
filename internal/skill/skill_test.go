package skill

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeSkillMD(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadFrontmatter_Basic(t *testing.T) {
	dir := t.TempDir()
	writeSkillMD(t, dir, "---\nname: handoff\ndescription: Hand off a session\n---\n\n# Body\n")

	fm, err := ReadFrontmatter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fm.Name != "handoff" {
		t.Errorf("Name = %q", fm.Name)
	}
	if fm.Description != "Hand off a session" {
		t.Errorf("Description = %q", fm.Description)
	}
}

func TestReadFrontmatter_NoFile(t *testing.T) {
	_, err := ReadFrontmatter(t.TempDir())
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("want ErrNotExist, got %v", err)
	}
}

func TestReadFrontmatter_MissingOpenDelim(t *testing.T) {
	dir := t.TempDir()
	writeSkillMD(t, dir, "name: handoff\n---\n")
	if _, err := ReadFrontmatter(dir); err == nil {
		t.Error("expected error for missing opening ---")
	}
}

func TestReadFrontmatter_Unterminated(t *testing.T) {
	dir := t.TempDir()
	writeSkillMD(t, dir, "---\nname: handoff\n")
	if _, err := ReadFrontmatter(dir); err == nil {
		t.Error("expected error for unterminated frontmatter")
	}
}
