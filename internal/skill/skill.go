// Package skill parses the YAML frontmatter of a SKILL.md file.
//
// A SKILL.md begins with:
//
//	---
//	name: handoff
//	description: ...
//	---
//
// csk only needs the `name` field, but the rest of the document is preserved.
package skill

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName is the conventional skill manifest filename.
const FileName = "SKILL.md"

// Frontmatter is the YAML header. Only fields csk reads are typed; everything
// else round-trips through Rest.
type Frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
}

// ReadFrontmatter reads <dir>/SKILL.md and parses its YAML frontmatter.
// Returns os.ErrNotExist if no SKILL.md is present.
func ReadFrontmatter(dir string) (*Frontmatter, error) {
	path := filepath.Join(dir, FileName)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, errors.New(path + ": missing YAML frontmatter opening (---)")
	}
	var body strings.Builder
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			var fm Frontmatter
			if err := yaml.Unmarshal([]byte(body.String()), &fm); err != nil {
				return nil, err
			}
			return &fm, nil
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New(path + ": unterminated YAML frontmatter")
}
