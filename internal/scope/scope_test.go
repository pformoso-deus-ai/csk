package scope

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve_ForceGlobalAndProjectConflict(t *testing.T) {
	if _, err := Resolve("/cwd", "/home", true, true); err == nil {
		t.Error("expected conflict error")
	}
}

func TestResolve_ForceGlobal(t *testing.T) {
	s, err := Resolve("/cwd", "/home", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != Global {
		t.Errorf("Kind = %v", s.Kind)
	}
	if s.Root != filepath.Join("/home", ".claude") {
		t.Errorf("Root = %q", s.Root)
	}
}

func TestResolve_ForceProject(t *testing.T) {
	s, err := Resolve("/cwd", "/home", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != Project {
		t.Errorf("Kind = %v", s.Kind)
	}
	if s.Root != filepath.Join("/cwd", ".claude") {
		t.Errorf("Root = %q", s.Root)
	}
}

func TestResolve_AutoDetectsProject(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	// Seed a project manifest so auto-detection picks Project.
	if err := os.MkdirAll(filepath.Join(cwd, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".claude", "skills.toml"), []byte("version = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Resolve(cwd, home, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != Project {
		t.Errorf("Kind = %v, want Project", s.Kind)
	}
}

func TestResolve_AutoDefaultsToGlobal(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	s, err := Resolve(cwd, home, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if s.Kind != Global {
		t.Errorf("Kind = %v, want Global", s.Kind)
	}
}

func TestPathsAreUnderRoot(t *testing.T) {
	s, _ := Resolve("/cwd", "/home", true, false)
	want := []string{s.ManifestPath, s.LockfilePath, s.CacheDir, s.SkillsDir, s.ProcLockPath}
	for _, p := range want {
		if filepath.Dir(p) != s.Root && p != s.Root {
			t.Errorf("path %q not under root %q", p, s.Root)
		}
	}
}
