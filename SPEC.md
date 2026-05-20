# Spec: `csk` — a skill dependency manager for Claude Code

## Context

Claude Code has two ways to load a skill: **plugins** (distributed via marketplace, with update tooling) and **personal skills** (loose files in `~/.claude/skills/<name>/` or `.claude/skills/<name>/`). Plugins have a distribution story; personal skills don't. The current workflow for someone who develops a personal skill in its own git repo is: clone manually, copy or symlink into `~/.claude/skills/`, remember to `git pull` later. There is no lockfile, no manifest, no way to declare a reproducible set of skills, no way to share that set across machines.

This spec describes **`csk`** (working name — "Claude Skill Kit"): a lightweight, git-native dependency manager for personal Claude Code skills, modeled on `uv`/`cargo` rather than `pip`. Its only job is to take a declarative list of skill sources, install them as junctions/symlinks into the locations Claude Code already reads, and keep that state reproducible.

## Goals

- Declarative: a manifest file lists desired skills + sources; the tool reconciles.
- Reproducible: a lockfile pins each skill to an exact commit SHA. `csk install` (no args) on a fresh machine produces an identical environment.
- Git-native: skills are git repos. No central registry in v1.
- Zero coupling to Claude Code: the tool writes to the same `~/.claude/skills/` and `.claude/skills/` paths Claude Code reads. Claude Code requires no awareness of `csk`.
- Coexistence: skills installed by hand (the current state of the world for most users) can be adopted into a manifest without reinstalling.

## Non-goals (v1)

- A central registry à la PyPI. Git URLs only.
- Plugin management — that lives in Claude Code's plugin system.
- Skill scaffolding / templates (separate tool).
- Version resolution with semver constraints. v1 pins by commit SHA or by "follow this branch". A SAT solver is overkill.
- Sandboxing / supply-chain attestation. Git's trust model is the baseline.

## Concepts

- **Source**: a git repo URL plus an optional subdirectory (for monorepos containing multiple skills) and an optional ref (branch / tag / commit). Example: `https://github.com/pablo/handoff-skill.git` with no subdir, ref `main`.
- **Skill**: the deployable unit. Identified by the `name:` field in its `SKILL.md` frontmatter. One source can produce one skill (v1) — multi-skill sources are out of scope.
- **Scope**: `global` (installs to `~/.claude/skills/`) or `project` (installs to `<cwd>/.claude/skills/`). Scope is a property of *which manifest* declares the skill, not of the skill itself.
- **Manifest**: declarative list of sources. One per scope. TOML.
- **Lockfile**: resolved manifest with pinned commit SHAs. One per scope. TOML.
- **Cache**: directory where repos are cloned and updated. Junctions in `skills/` point into the cache.

## File layout

```
# Global scope
~/.claude/skills.toml              # manifest (user-edited)
~/.claude/skills.lock              # lockfile (tool-managed; commit to dotfiles repo)
~/.claude/skills-cache/<name>/     # clones — full git working trees
~/.claude/skills/<name>            # junction → ../skills-cache/<name>/[subdir]

# Project scope (mirror, rooted at project)
<project>/.claude/skills.toml
<project>/.claude/skills.lock
<project>/.claude/skills-cache/<name>/
<project>/.claude/skills/<name>
```

The `skills-cache/` directory is the source of truth for installed bits; `skills/` is just the surface Claude Code reads. This split lets the user `git pull` directly in a cache entry while developing, or let `csk` manage it.

## Manifest format (`skills.toml`)

```toml
# Manifest version. Lets the format evolve without breaking old files.
version = 1

# Each entry under [skills] is one declared skill.
# The TOML key is the local install name (becomes the junction name).
# Free-form within filesystem constraints.

[skills.handoff]
source = "https://github.com/pablo/handoff-skill.git"
ref = "main"                      # branch / tag / commit. Default: "main".
# Optional fields:
# subdir = "packages/handoff"     # for monorepos. Default: repo root.
# scope  = "global"                # informational only — derived from manifest location.
```

Minimum required field: `source`. Everything else has a default.

## Lockfile format (`skills.lock`)

```toml
version = 1
generated = "2026-05-20T18:23:00Z"

[[skill]]
name = "handoff"
source = "https://github.com/pablo/handoff-skill.git"
ref = "main"
commit = "a1b2c3d4e5f6789..."     # the pinned SHA
subdir = ""                        # empty string = repo root
```

The lockfile is checked into the user's dotfiles repo (global) or project repo (project scope). It is the contract between machines.

## CLI surface

All commands operate on the manifest in the **current scope**. Default scope is auto-detected: if `<cwd>/.claude/skills.toml` exists, project; otherwise global. `--global` and `--project` flags force.

```
csk init                          # create empty manifest (and lockfile) in current scope
csk add <source> [--name NAME] [--ref REF] [--subdir PATH]
                                  # add entry to manifest, resolve, update lockfile, install
csk remove <name>                  # remove entry, drop junction, optionally prune cache
csk install                        # reconcile cache + junctions to match lockfile
csk update [name ...]              # for each (or all): re-resolve ref, update lockfile, sync cache
csk lock                           # re-resolve manifest, rewrite lockfile, no install
csk sync                           # alias for `install` — explicit "match the lockfile"
csk list                           # show installed skills, their sources, pinned commits, drift
csk adopt <name> --source URL [--ref REF] [--subdir PATH]
                                  # take a skill already present at ~/.claude/skills/<name>/,
                                  # register it in the manifest, replace dir with junction
csk doctor                         # diagnose: missing junctions, broken cache, manifest/lockfile drift
```

Mutating commands (`add`, `remove`, `update`, `install`, `sync`) always end by rewriting the lockfile. Read-only commands (`list`, `doctor`) never touch state.

## Command behavior

**`csk add <source>`** — clone the source into `skills-cache/<name>/`, resolve `ref` to a commit SHA, write the entry to manifest, write to lockfile, create the junction. `<name>` is inferred from the repo basename if `--name` is omitted, or read from the cloned repo's `SKILL.md` frontmatter if present and unambiguous.

**`csk install`** — read lockfile. For each entry: if cache dir absent, clone and `git checkout <commit>`. If cache dir present but at a different commit, `git fetch` + `git checkout <commit>`. Create or refresh junction. Idempotent; safe to re-run.

**`csk update <name>`** — for the named skill: in its cache dir, `git fetch`, resolve `ref` to the latest commit on that ref, update lockfile entry, `git checkout` to the new commit. `csk update` with no args = all skills.

**`csk adopt`** — handles the migration case where the user already installed a skill by hand. Verifies that the current `skills/<name>/` exists and contains a valid `SKILL.md`. Asks (or `--yes`) before replacing it with a junction. Clones source into cache. Diff-checks that the cache content matches what was there. If mismatch, surface and stop unless `--force`.

**`csk doctor`** — checks: (a) every lockfile entry has a cache dir at the right commit; (b) every cache dir has a corresponding junction; (c) every junction's target exists; (d) every manifest entry has a lockfile entry. Prints a punch list; never modifies.

## Platform support

- **Linux / macOS**: symlinks.
- **Windows**: directory junctions (`mklink /J` / `New-Item -ItemType Junction`). Symlinks require admin or developer mode; junctions don't. Junctions are the right primitive here — same target semantics, no privilege escalation.

The tool detects platform and picks the right primitive transparently.

## Behavior in edge cases

- **Concurrent install/update**: simple lockfile (`skills.toml.lock-file`) at the scope root. Conflicting operations fail loud.
- **Cache contains uncommitted changes** (user was developing in the cache dir): `csk update` and `csk install` refuse to overwrite. Surface the dirty state. Provide `--discard` for explicit override.
- **Source URL changes for an existing entry**: `csk add` with a new source for an existing name = error; user must `csk remove` first or pass `--force`. Avoids silent re-pointing.
- **Skill name in `SKILL.md` frontmatter differs from manifest key**: warn on install. The manifest key wins for the junction path; Claude Code uses the frontmatter `name` for the trigger surface. The discrepancy is legal but confusing — surface it.
- **Lockfile committed across machines with different OS**: lockfile is OS-independent (just URLs and SHAs). Junctions vs symlinks are decided at install time, not stored.

## Migration / adoption story

A user with N skills already installed by hand:

```
$ csk init                                # creates ~/.claude/skills.toml
$ csk adopt handoff --source https://github.com/pablo/handoff-skill.git
$ csk adopt codegraph-helper --source https://github.com/pablo/codegraph-helper.git
$ csk list
```

After adoption, the user can commit `~/.claude/skills.toml` and `~/.claude/skills.lock` to their dotfiles repo. On a fresh machine: `git clone dotfiles && csk install` and they have the same skill environment.

## Implementation language

Go, single static binary. Cross-platform, no runtime dependency on the user's machine beyond `git` on `PATH`.

## v1 implementation decisions

These are decisions made during scaffolding that flesh out the spec:

- **Git operations**: shell out to the user's installed `git`. Defers SSH/HTTPS/credential-helper handling to existing config.
- **Junction vs symlink**: strictly OS-based. Windows = junction via `mklink /J`. Unix = symlink via `os.Symlink`. Recomputed at install time; never stored.
- **Name resolution precedence**: `--name` flag > `SKILL.md` frontmatter `name:` > repo basename (with `.git` stripped).
- **Concurrency lock**: advisory dot-file `skills.toml.lock` at scope root, held for the duration of any mutating command. Conflicting invocations fail loud, no retry.
- **Dirty cache**: any non-empty `git status --porcelain` blocks `update`/`install` unless `--discard` is passed.
- **`csk adopt` diff**: recursive byte-equal comparison between cloned cache and existing `skills/<name>/`. Exact match → swap to junction; mismatch → list divergent files and bail unless `--force`.
- **Exit codes**: 0 success, 1 user error, 2 environment error.
- **`csk doctor`**: read-only in v1.
- **Output**: plain text; ANSI color only when stdout is a TTY. No structured / JSON output in v1.
- **No telemetry, no auto-update check.** Zero network beyond `git`.

## Open questions (deferred)

1. **Name**: `csk` is the working name. Renaming the binary is cheap pre-1.0.
2. **Scope precedence** when a skill exists in both global and project scope: defer to Claude Code's behavior; document, do not enforce.
3. **Lockfile commit posture**: tool is agnostic. We don't touch `.gitignore`; README recommends committing.
4. **`csk run <skill>` style commands**: out of scope. No Claude-Code-CLI hook to invoke skills externally today.
5. **Plugin-format target**: separate route. Plugin packaging belongs in plugin tooling; `csk` stays on the dev-loop side.

## Acceptance criteria for v1

- [ ] `csk init`, `csk add`, `csk install`, `csk update`, `csk remove`, `csk list`, `csk adopt`, `csk doctor` all implemented and documented.
- [ ] Manifest + lockfile format matches the schemas above and is versioned (`version = 1`).
- [ ] On Windows, junctions; on macOS/Linux, symlinks. Decided automatically.
- [ ] `csk install` is idempotent — running twice in a row produces no changes.
- [ ] `csk adopt` does not lose user data: refuses to replace a non-junction directory unless content matches the cloned source, or `--force` is passed.
- [ ] A round-trip test: `csk init` → `csk add <real repo>` → delete `skills/` and lockfile → check `skills.toml` only → `csk install` → state is restored to the same SHA.
