// Package scope resolves which scope (global vs project) csk should act on
// and provides absolute paths for the manifest, lockfile, cache, and skills
// directories under that scope.
package scope

import (
	"errors"
	"os"
	"path/filepath"
)

// Kind is the scope kind: global ($HOME/.claude) or project (cwd/.claude).
type Kind int

const (
	Unknown Kind = iota
	Global
	Project
)

func (k Kind) String() string {
	switch k {
	case Global:
		return "global"
	case Project:
		return "project"
	default:
		return "unknown"
	}
}

// Scope is the resolved set of paths for one scope kind.
type Scope struct {
	Kind         Kind
	Root         string // <root>/.claude
	ManifestPath string // <root>/.claude/skills.toml
	LockfilePath string // <root>/.claude/skills.lock
	CacheDir     string // <root>/.claude/skills-cache
	SkillsDir    string // <root>/.claude/skills
	ProcLockPath string // <root>/.claude/skills.toml.lock  (concurrency guard)
}

// Resolve decides which scope to use.
//
// Precedence:
//  1. If forceGlobal is true → Global.
//  2. If forceProject is true → Project (rooted at cwd).
//  3. If <cwd>/.claude/skills.toml exists → Project.
//  4. Otherwise → Global.
func Resolve(cwd, home string, forceGlobal, forceProject bool) (*Scope, error) {
	if forceGlobal && forceProject {
		return nil, errors.New("--global and --project are mutually exclusive")
	}
	switch {
	case forceGlobal:
		return forHome(home), nil
	case forceProject:
		return forProject(cwd), nil
	}
	projectManifest := filepath.Join(cwd, ".claude", "skills.toml")
	if _, err := os.Stat(projectManifest); err == nil {
		return forProject(cwd), nil
	}
	return forHome(home), nil
}

func forHome(home string) *Scope {
	root := filepath.Join(home, ".claude")
	return build(Global, root)
}

func forProject(cwd string) *Scope {
	root := filepath.Join(cwd, ".claude")
	return build(Project, root)
}

func build(k Kind, root string) *Scope {
	return &Scope{
		Kind:         k,
		Root:         root,
		ManifestPath: filepath.Join(root, "skills.toml"),
		LockfilePath: filepath.Join(root, "skills.lock"),
		CacheDir:     filepath.Join(root, "skills-cache"),
		SkillsDir:    filepath.Join(root, "skills"),
		ProcLockPath: filepath.Join(root, "skills.toml.lock"),
	}
}
