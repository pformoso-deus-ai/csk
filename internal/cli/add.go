package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/cache"
	"github.com/pformoso-deus-ai/csk/internal/lockfile"
	"github.com/pformoso-deus-ai/csk/internal/manifest"
	"github.com/pformoso-deus-ai/csk/internal/procguard"
	"github.com/pformoso-deus-ai/csk/internal/registry"
	"github.com/pformoso-deus-ai/csk/internal/skill"
)

func newAddCmd() *cobra.Command {
	var (
		nameFlag   string
		refFlag    string
		subdirFlag string
		force      bool
	)
	cmd := &cobra.Command{
		Use:   "add <source>",
		Short: "Clone a git source and register it as a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			if source == "" {
				return userErr(errors.New("source must not be empty"))
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

			ctxAdd := cmd.Context()
			if ctxAdd == nil {
				ctxAdd = context.Background()
			}

			// If <source> isn't a URL/path, treat it as a registry name and
			// resolve via the published index. The user-typed name becomes
			// the install name unless --name overrides it.
			if registry.LooksLikeRegistryName(source) {
				idx, ferr := registry.New().Fetch(ctxAdd, false)
				if ferr != nil {
					return envErr(fmt.Errorf("looking up %q in registry: %w", source, ferr))
				}
				entry := idx.Find(source)
				if entry == nil {
					return userErr(fmt.Errorf("skill %q not found in registry — pass a git URL instead, or check `csk search`", source))
				}
				if nameFlag == "" {
					nameFlag = entry.Name
				}
				if subdirFlag == "" {
					subdirFlag = entry.Subdir
				}
				if refFlag == "" && entry.DefaultRef != "" {
					refFlag = entry.DefaultRef
				}
				source = entry.Source
				fmt.Fprintf(cmd.OutOrStdout(), "csk: resolved %s → %s\n", entry.Name, source)
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
			lf, err := lockfile.Load(s.LockfilePath)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return userErr(err)
				}
				lf = lockfile.New()
			}

			ref := refFlag
			if ref == "" {
				ref = manifest.DefaultRef
			}

			// If this source is already declared, lock onto that name.
			existingNameForSource := ""
			for k, v := range mf.Skills {
				if v.Source == source {
					existingNameForSource = k
					break
				}
			}

			// Tentative name: --name > existing-by-source > basename
			tentative := nameFlag
			if tentative == "" {
				if existingNameForSource != "" {
					tentative = existingNameForSource
				} else {
					tentative = skill.InferNameFromSource(source)
				}
			}
			if tentative == "" {
				return userErr(errors.New("could not infer skill name from source; pass --name"))
			}

			// Conflict at tentative name (different-source case).
			if existing, ok := mf.Skills[tentative]; ok {
				if existing.Source != source && !force {
					return userErr(fmt.Errorf("skill %q already declared with a different source (use --force to override)", tentative))
				}
			}

			ctx := ctxAdd

			// Track whether the cache dir already existed before this invocation.
			// If we cloned it as part of this `csk add` and then fail validation
			// downstream, roll back the clone so a corrected retry isn't blocked.
			cachePreExisted := false
			if _, statErr := os.Stat(cache.CacheDir(s, tentative)); statErr == nil {
				cachePreExisted = true
			}

			// Clone or fetch, resolve ref → commit.
			plan := cache.Plan{Name: tentative, Source: source, Ref: ref, Subdir: subdirFlag}
			commit, err := cache.Resolve(ctx, s, plan)
			if err != nil {
				if !cachePreExisted {
					_ = os.RemoveAll(cache.CacheDir(s, tentative))
				}
				return envErr(err)
			}
			plan.Commit = commit

			// SKILL.md-based name upgrade, only on a fresh add (no --name, no
			// existing source match).
			finalName := tentative

			// rollback removes the cache dir we just created if and only if it
			// didn't exist before this add. If the SKILL.md upgrade renamed
			// the dir, we remove the renamed location instead.
			rollback := func() {
				if cachePreExisted {
					return
				}
				_ = os.RemoveAll(cache.CacheDir(s, tentative))
				if finalName != tentative {
					_ = os.RemoveAll(cache.CacheDir(s, finalName))
				}
			}
			if nameFlag == "" && existingNameForSource == "" {
				if fm, ferr := skill.ReadFrontmatter(cache.LinkTarget(s, plan)); ferr == nil && fm.Name != "" && fm.Name != tentative {
					if _, ok := mf.Skills[fm.Name]; ok {
						rollback()
						return userErr(fmt.Errorf("SKILL.md declares name %q which is already in the manifest under a different source", fm.Name))
					}
					newDir := cache.CacheDir(s, fm.Name)
					if _, err := os.Stat(newDir); err == nil {
						rollback()
						return userErr(fmt.Errorf("SKILL.md declares name %q but cache dir %s already exists", fm.Name, newDir))
					}
					if err := os.Rename(cache.CacheDir(s, tentative), newDir); err != nil {
						rollback()
						return envErr(err)
					}
					finalName = fm.Name
					plan.Name = finalName
				}
			}

			// Require SKILL.md at the final target.
			if _, ferr := skill.ReadFrontmatter(cache.LinkTarget(s, plan)); ferr != nil {
				rollback()
				if errors.Is(ferr, os.ErrNotExist) {
					return userErr(fmt.Errorf("no SKILL.md at %s — is this a Claude Code skill?", cache.LinkTarget(s, plan)))
				}
				return userErr(ferr)
			}

			if err := cache.Reconcile(ctx, s, plan, false); err != nil {
				rollback()
				return envErr(err)
			}

			mf.Skills[finalName] = manifest.Entry{Source: source, Ref: refFlag, Subdir: subdirFlag}
			lf.Upsert(lockfile.Entry{Name: finalName, Source: source, Ref: ref, Commit: commit, Subdir: subdirFlag})
			if err := mf.Save(s.ManifestPath); err != nil {
				rollback()
				return envErr(err)
			}
			if err := lf.Save(s.LockfilePath); err != nil {
				rollback()
				return envErr(err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "csk: added %s @ %s (%s)\n", finalName, ref, shortSHA(commit))
			return nil
		},
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "override the install name")
	cmd.Flags().StringVar(&refFlag, "ref", "", "branch, tag, or commit to pin (default: main)")
	cmd.Flags().StringVar(&subdirFlag, "subdir", "", "subdirectory inside the repo to expose")
	cmd.Flags().BoolVar(&force, "force", false, "allow replacing an existing entry with a different source")
	return cmd
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}
