// Package manifest reads and writes skills.toml.
package manifest

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

const Version = 1

// File is the top-level skills.toml document.
type File struct {
	Version int              `toml:"version"`
	Skills  map[string]Entry `toml:"skills"`
}

// Entry is one declared skill.
//
//	[skills.<key>]
//	source = "https://github.com/..."
//	ref    = "main"      # optional, default "main"
//	subdir = "..."        # optional, default ""
//
// The map key in File.Skills is the local install name (= junction name).
type Entry struct {
	Source string `toml:"source"`
	Ref    string `toml:"ref,omitempty"`
	Subdir string `toml:"subdir,omitempty"`
}

// DefaultRef is the ref used when none is specified.
const DefaultRef = "main"

// New returns an empty, valid manifest.
func New() *File {
	return &File{Version: Version, Skills: map[string]Entry{}}
}

// Load reads and validates a manifest from disk.
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

// Save atomically writes the manifest to disk (write-temp-then-rename).
func (f *File) Save(path string) error {
	// TODO(v1): implement atomic write
	b, err := toml.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Validate enforces manifest invariants.
func (f *File) Validate() error {
	if f.Version != Version {
		return fmt.Errorf("unsupported manifest version %d (want %d)", f.Version, Version)
	}
	for name, e := range f.Skills {
		if name == "" {
			return fmt.Errorf("skill key must not be empty")
		}
		if e.Source == "" {
			return fmt.Errorf("skill %q: source is required", name)
		}
	}
	return nil
}

// RefOrDefault returns e.Ref if set, otherwise DefaultRef.
func (e Entry) RefOrDefault() string {
	if e.Ref == "" {
		return DefaultRef
	}
	return e.Ref
}
