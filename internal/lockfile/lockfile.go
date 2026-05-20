// Package lockfile reads and writes skills.lock.
package lockfile

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/pformoso/csk/internal/atomicfile"
)

const Version = 1

// File is the top-level skills.lock document.
type File struct {
	Version   int       `toml:"version"`
	Generated time.Time `toml:"generated"`
	Skills    []Entry   `toml:"skill"`
}

// Entry is one pinned skill in the lockfile.
type Entry struct {
	Name   string `toml:"name"`
	Source string `toml:"source"`
	Ref    string `toml:"ref"`
	Commit string `toml:"commit"`
	Subdir string `toml:"subdir"`
}

// New returns an empty, valid lockfile stamped at now.
func New() *File {
	return &File{Version: Version, Generated: time.Now().UTC()}
}

// Load parses a lockfile from disk.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := toml.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &f, nil
}

// Save writes the lockfile, stamping Generated to now, with entries sorted
// by name for stable diffs.
func (f *File) Save(path string) error {
	f.Generated = time.Now().UTC()
	sort.Slice(f.Skills, func(i, j int) bool { return f.Skills[i].Name < f.Skills[j].Name })
	b, err := toml.Marshal(f)
	if err != nil {
		return err
	}
	return atomicfile.WriteFile(path, b, 0o644)
}

// Validate enforces lockfile invariants.
func (f *File) Validate() error {
	if f.Version != Version {
		return fmt.Errorf("unsupported lockfile version %d (want %d)", f.Version, Version)
	}
	seen := map[string]bool{}
	for _, e := range f.Skills {
		if e.Name == "" {
			return fmt.Errorf("skill entry: name required")
		}
		if seen[e.Name] {
			return fmt.Errorf("skill %q: duplicate entry", e.Name)
		}
		seen[e.Name] = true
		if e.Source == "" {
			return fmt.Errorf("skill %q: source required", e.Name)
		}
		if e.Commit == "" {
			return fmt.Errorf("skill %q: commit required", e.Name)
		}
	}
	return nil
}

// Find returns the entry with the given name, or nil.
func (f *File) Find(name string) *Entry {
	for i := range f.Skills {
		if f.Skills[i].Name == name {
			return &f.Skills[i]
		}
	}
	return nil
}

// Upsert replaces an existing entry by name or appends a new one.
func (f *File) Upsert(e Entry) {
	for i := range f.Skills {
		if f.Skills[i].Name == e.Name {
			f.Skills[i] = e
			return
		}
	}
	f.Skills = append(f.Skills, e)
}

// Remove drops an entry by name. Returns true if it existed.
func (f *File) Remove(name string) bool {
	for i := range f.Skills {
		if f.Skills[i].Name == name {
			f.Skills = append(f.Skills[:i], f.Skills[i+1:]...)
			return true
		}
	}
	return false
}
