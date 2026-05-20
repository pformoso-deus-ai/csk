package manifest

import (
	"path/filepath"
	"testing"
)

func TestNewAndValidate(t *testing.T) {
	m := New()
	if m.Version != Version {
		t.Errorf("Version = %d, want %d", m.Version, Version)
	}
	if m.Skills == nil {
		t.Error("Skills map must be non-nil")
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "skills.toml")

	in := New()
	in.Skills["handoff"] = Entry{
		Source: "https://github.com/pablo/handoff-skill.git",
		Ref:    "main",
	}
	in.Skills["codegraph"] = Entry{
		Source: "https://github.com/pablo/codegraph.git",
		Subdir: "pkg/skill",
	}
	if err := in.Save(p); err != nil {
		t.Fatal(err)
	}

	out, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(out.Skills))
	}
	if out.Skills["handoff"].Source != in.Skills["handoff"].Source {
		t.Errorf("handoff source mismatch")
	}
	if out.Skills["codegraph"].Subdir != "pkg/skill" {
		t.Errorf("codegraph subdir = %q", out.Skills["codegraph"].Subdir)
	}
}

func TestValidate_RejectsMissingSource(t *testing.T) {
	m := New()
	m.Skills["x"] = Entry{} // no source
	if err := m.Validate(); err == nil {
		t.Error("expected validation error for missing source")
	}
}

func TestValidate_RejectsBadVersion(t *testing.T) {
	m := &File{Version: 99, Skills: map[string]Entry{}}
	if err := m.Validate(); err == nil {
		t.Error("expected validation error for bad version")
	}
}

func TestRefOrDefault(t *testing.T) {
	if got := (Entry{}).RefOrDefault(); got != DefaultRef {
		t.Errorf("empty Ref → %q, want %q", got, DefaultRef)
	}
	if got := (Entry{Ref: "v2"}).RefOrDefault(); got != "v2" {
		t.Errorf("Ref=v2 → %q", got)
	}
}
