// Package gitx is a thin wrapper around the user's installed `git` binary.
// csk shells out rather than embedding go-git so that SSH keys, credential
// helpers, and protocol choices are handled by the user's existing config.
package gitx

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner runs git subcommands. The default is exec.CommandContext.
// Tests can substitute a fake.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

// Default is the package-level runner.
var Default Runner = execRunner{}

// Clone runs `git clone <source> <dest>`.
func Clone(ctx context.Context, source, dest string) error {
	_, err := Default.Run(ctx, "", "clone", "--", source, dest)
	return err
}

// Fetch runs `git fetch --tags origin` in dir.
func Fetch(ctx context.Context, dir string) error {
	_, err := Default.Run(ctx, dir, "fetch", "--tags", "origin")
	return err
}

// Checkout runs `git checkout <ref>` in dir.
func Checkout(ctx context.Context, dir, ref string) error {
	_, err := Default.Run(ctx, dir, "checkout", ref)
	return err
}

// ResolveRef returns the commit SHA that <ref> currently points to inside dir.
// Equivalent to `git rev-parse <ref>^{commit}`.
func ResolveRef(ctx context.Context, dir, ref string) (string, error) {
	out, err := Default.Run(ctx, dir, "rev-parse", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// IsDirty reports whether the working tree at dir has uncommitted changes.
// Equivalent to `git status --porcelain` being non-empty.
func IsDirty(ctx context.Context, dir string) (bool, error) {
	out, err := Default.Run(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

// HardReset runs `git reset --hard && git clean -fd` inside dir.
// Use only when the caller has confirmed --discard.
func HardReset(ctx context.Context, dir string) error {
	if _, err := Default.Run(ctx, dir, "reset", "--hard"); err != nil {
		return err
	}
	_, err := Default.Run(ctx, dir, "clean", "-fd")
	return err
}

// HeadCommit returns the commit SHA at HEAD.
func HeadCommit(ctx context.Context, dir string) (string, error) {
	return ResolveRef(ctx, dir, "HEAD")
}
