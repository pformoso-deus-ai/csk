# csk — Claude Skill Kit

A git-native dependency manager for **personal Claude Code skills**.

Plugins have a distribution story. Personal skills don't. `csk` is to skills what `cargo` / `uv` is to libraries: a declarative manifest, a pinned lockfile, and a reproducible `csk install` on any machine.

> Status: **scaffolding**. CLI surface is stubbed; commands are not yet implemented.

## Concept

```
~/.claude/skills.toml              # manifest (you edit this)
~/.claude/skills.lock              # lockfile (csk writes this — commit to your dotfiles)
~/.claude/skills-cache/<name>/     # full git clone; source of truth
~/.claude/skills/<name>            # symlink / junction → ../skills-cache/<name>/[subdir]
```

Same layout under `<project>/.claude/` for project-scoped skills.

## Commands (planned)

| Command | What it does |
|---|---|
| `csk init` | Create empty manifest + lockfile in the current scope |
| `csk add <git-url>` | Clone, resolve ref → SHA, write manifest + lockfile, install |
| `csk remove <name>` | Remove entry, drop junction, optionally prune cache |
| `csk install` | Reconcile cache + junctions to match lockfile (idempotent) |
| `csk sync` | Alias for `install` |
| `csk update [name ...]` | Re-resolve ref, update lockfile, sync cache |
| `csk lock` | Re-resolve manifest, rewrite lockfile, no install |
| `csk list` | Show installed skills + drift state |
| `csk adopt <name> --source URL` | Register a hand-installed skill into the manifest |
| `csk doctor` | Diagnose cache/junction/lockfile drift (read-only) |

Scope is auto-detected: project if `./.claude/skills.toml` exists, otherwise global. Force with `--global` / `--project`.

## Manifest (`skills.toml`)

```toml
version = 1

[skills.handoff]
source = "https://github.com/pablo/handoff-skill.git"
ref = "main"           # branch, tag, or commit. Default: "main".
# subdir = "packages/handoff"  # optional, for monorepos.
```

## Lockfile (`skills.lock`)

Tool-managed. One `[[skill]]` block per entry, with `commit = "<full SHA>"`. OS-independent — commit it across machines.

## Building from source

`csk` is written in Go. You need Go 1.23+ and `git` on `PATH`.

```sh
go build -o csk ./cmd/csk
./csk --help
```

Cross-compile:

```sh
GOOS=windows GOARCH=amd64 go build -o csk.exe ./cmd/csk
GOOS=darwin  GOARCH=arm64 go build -o csk     ./cmd/csk
GOOS=linux   GOARCH=amd64 go build -o csk     ./cmd/csk
```

## Platform notes

- Linux / macOS: skills are exposed as symlinks.
- Windows: skills are exposed as directory junctions (`mklink /J`). No admin / developer-mode required.

Decided automatically at install time. The lockfile is OS-independent.

## Design

See [`SPEC.md`](SPEC.md) for the full design document.

## License

MIT — see [LICENSE](LICENSE).
