// Package cli wires the cobra command tree.
package cli

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/scope"
)

// Version is set via -ldflags at release time. Default is a dev marker.
var Version = "0.0.0-dev"

// Global flags, populated by cobra and consumed via the helpers below.
var (
	flagGlobal  bool
	flagProject bool
)

// NewRootCmd builds the root command and attaches all subcommands.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "csk",
		Short:         "Git-native dependency manager for personal Claude Code skills",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&flagGlobal, "global", false, "operate on the global scope (~/.claude)")
	root.PersistentFlags().BoolVar(&flagProject, "project", false, "operate on the project scope (./.claude)")

	root.AddCommand(
		newInitCmd(),
		newAddCmd(),
		newRemoveCmd(),
		newInstallCmd(),
		newSyncCmd(),
		newUpdateCmd(),
		newLockCmd(),
		newListCmd(),
		newAdoptCmd(),
		newDoctorCmd(),
		newUpgradeCmd(),
	)
	return root
}

// resolveScope is the helper every subcommand uses to figure out which scope
// to act on. It honors --global / --project, and otherwise sniffs cwd.
func resolveScope() (*scope.Scope, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return scope.Resolve(cwd, home, flagGlobal, flagProject)
}

// errNotImplemented is returned by every command stub until its body is
// written. Distinct from a runtime/user error so the test suite can detect
// "still scaffold" vs "regression".
var errNotImplemented = errors.New("not implemented")
