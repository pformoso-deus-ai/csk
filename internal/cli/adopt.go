package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pformoso/csk/internal/cache"
	"github.com/pformoso/csk/internal/gitx"
	"github.com/pformoso/csk/internal/link"
	"github.com/pformoso/csk/internal/lockfile"
	"github.com/pformoso/csk/internal/manifest"
	"github.com/pformoso/csk/internal/procguard"
	"github.com/pformoso/csk/internal/skill"
)

func newAdoptCmd() *cobra.Command {
	var (
		source string
		ref    string
		subdir string
		yes    bool
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "adopt <name>",
		Short: "Register an existing hand-installed skill into the manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if source == "" {
				return userErr(errors.New("--source is required"))
			}
			name := args[0]
			if name == "" {
				return userErr(errors.New("name must not be empty"))
			}

			s, err := resolveScope()
			if err != nil {
				return userErr(err)
			}
			if _, err := os.Stat(s.ManifestPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return userErr(fmt.Errorf("no manifest at %s — run `csk init` first", s.ManifestPath))
				}
				return envErr(err)
			}

			g, err := procguard.Acquire(s.ProcLockPath)
			if err != nil {
				return classifyProcguard(err)
			}
			defer g.Unlock()

			mf, err := manifest.Load(s.ManifestPath)
			if err != nil {
				return userErr(err)
			}
			lf, err := loadLockfileOrEmpty(s.LockfilePath)
			if err != nil {
				return userErr(err)
			}

			if _, ok := mf.Skills[name]; ok {
				return userErr(fmt.Errorf("skill %q is already in the manifest — use `csk update` or `csk remove`", name))
			}

			// Validate the existing directory we're being asked to adopt.
			existing := cache.LinkPath(s, name) // <skills>/<name>
			fi, lerr := os.Lstat(existing)
			if errors.Is(lerr, os.ErrNotExist) {
				return userErr(fmt.Errorf("no directory at %s — nothing to adopt", existing))
			}
			if lerr != nil {
				return envErr(lerr)
			}
			if fi.Mode()&os.ModeSymlink != 0 || fi.Mode()&os.ModeIrregular != 0 {
				return userErr(fmt.Errorf("%s is already a link — use `csk add` instead of `csk adopt`", existing))
			}
			if !fi.IsDir() {
				return userErr(fmt.Errorf("%s is not a directory", existing))
			}
			if _, ferr := skill.ReadFrontmatter(existing); ferr != nil {
				return userErr(fmt.Errorf("%s does not contain a readable SKILL.md (%v)", existing, ferr))
			}

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if err := os.MkdirAll(s.CacheDir, 0o755); err != nil {
				return envErr(err)
			}

			// Clone the source into a scratch dir inside the cache. Same
			// filesystem as the final cache location, so the eventual rename
			// is atomic.
			scratch, err := os.MkdirTemp(s.CacheDir, ".csk-adopt-*")
			if err != nil {
				return envErr(err)
			}
			// git clone wants the dest to not exist.
			if err := os.Remove(scratch); err != nil {
				return envErr(err)
			}
			defer os.RemoveAll(scratch) // cleanup if we error out
			if err := gitx.Clone(ctx, source, scratch); err != nil {
				return envErr(err)
			}

			refResolved := ref
			if refResolved == "" {
				refResolved = manifest.DefaultRef
			}
			commit, err := gitx.ResolveRef(ctx, scratch, refResolved)
			if err != nil {
				return envErr(fmt.Errorf("resolve %s: %w", refResolved, err))
			}
			if err := gitx.Checkout(ctx, scratch, commit); err != nil {
				return envErr(err)
			}

			// SKILL.md must exist in the source too.
			scratchTarget := scratch
			if subdir != "" {
				scratchTarget = filepath.Join(scratch, subdir)
				if _, err := os.Stat(scratchTarget); errors.Is(err, os.ErrNotExist) {
					return userErr(fmt.Errorf("subdir %q does not exist in source at %s", subdir, refResolved))
				}
			}
			if _, err := skill.ReadFrontmatter(scratchTarget); err != nil {
				return userErr(fmt.Errorf("source has no SKILL.md at %s — is the --subdir wrong?", scratchTarget))
			}

			// Diff existing skills/<name> vs scratch + subdir.
			diffs, err := diffDirs(existing, scratchTarget)
			if err != nil {
				return envErr(err)
			}
			if len(diffs) > 0 {
				if !force {
					return userErr(fmt.Errorf("%s diverges from %s at %s — refusing to adopt without --force.\nFirst diverging files (up to 20):\n  %s",
						existing, source, refResolved, strings.Join(truncate(diffs, 20), "\n  ")))
				}
				if !yes {
					return userErr(errors.New("--force requires --yes to confirm a destructive adoption"))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "csk: warning: %d file(s) differ — overwriting existing skill with %s\n", len(diffs), refResolved)
			}

			// Swap. Replace skills/<name> with a junction to <cache>/<name>.
			finalCache := cache.CacheDir(s, name)
			if _, err := os.Stat(finalCache); err == nil {
				return userErr(fmt.Errorf("cache dir %s already exists — remove it first", finalCache))
			}
			if err := os.Rename(scratch, finalCache); err != nil {
				return envErr(err)
			}
			// Successfully renamed — disarm the cleanup so we don't blow it away.
			scratch = ""

			if err := os.RemoveAll(existing); err != nil {
				return envErr(err)
			}
			target := finalCache
			if subdir != "" {
				target = filepath.Join(finalCache, subdir)
			}
			if err := link.Ensure(target, existing); err != nil {
				return envErr(err)
			}

			// Register.
			mf.Skills[name] = manifest.Entry{Source: source, Ref: ref, Subdir: subdir}
			lf.Upsert(lockfile.Entry{Name: name, Source: source, Ref: refResolved, Commit: commit, Subdir: subdir})
			if err := mf.Save(s.ManifestPath); err != nil {
				return envErr(err)
			}
			if err := lf.Save(s.LockfilePath); err != nil {
				return envErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "csk: adopted %s @ %s (%s)\n", name, refResolved, shortSHA(commit))
			return nil
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "git URL of the skill (required)")
	cmd.Flags().StringVar(&ref, "ref", "", "branch, tag, or commit to pin (default: main)")
	cmd.Flags().StringVar(&subdir, "subdir", "", "subdirectory inside the repo")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the confirmation prompt for --force")
	cmd.Flags().BoolVar(&force, "force", false, "adopt even if local content diverges from the source")
	_ = cmd.MarkFlagRequired("source")
	return cmd
}

// diffDirs returns the sorted list of file paths (relative to each root)
// whose contents differ between a and b. Directories named ".git" anywhere
// in the tree are ignored. Returns nil if the trees are byte-identical.
func diffDirs(a, b string) ([]string, error) {
	loadSet := func(root string) (map[string][]byte, error) {
		m := map[string][]byte{}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			m[rel] = data
			return nil
		})
		return m, err
	}

	aSet, err := loadSet(a)
	if err != nil {
		return nil, err
	}
	bSet, err := loadSet(b)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var diffs []string
	for k, v := range aSet {
		seen[k] = true
		if bv, ok := bSet[k]; !ok || !bytes.Equal(v, bv) {
			diffs = append(diffs, k)
		}
	}
	for k := range bSet {
		if !seen[k] {
			diffs = append(diffs, k)
		}
	}
	sort.Strings(diffs)
	return diffs, nil
}

func truncate(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return append(s[:n:n], fmt.Sprintf("... and %d more", len(s)-n))
}
