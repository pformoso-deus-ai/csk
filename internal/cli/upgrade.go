package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/selfupdate"
)

func newUpgradeCmd() *cobra.Command {
	var (
		checkOnly bool
		force     bool
	)
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the csk binary itself to the latest GitHub release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			out := cmd.OutOrStdout()

			client := selfupdate.New()
			release, err := client.LatestRelease(ctx)
			if err != nil {
				return envErr(err)
			}
			current := Version
			latest := release.TagName
			fmt.Fprintf(out, "csk: current=%s latest=%s\n", current, latest)

			if selfupdate.SameVersion(current, latest) && !force {
				fmt.Fprintln(out, "csk: already up to date")
				return nil
			}
			if checkOnly {
				fmt.Fprintln(out, "csk: an upgrade is available (run `csk upgrade` to install)")
				return nil
			}

			asset, err := selfupdate.AssetForCurrentPlatform(release)
			if err != nil {
				return userErr(err)
			}
			sums, err := selfupdate.FindChecksumsAsset(release)
			if err != nil {
				return userErr(err)
			}
			fmt.Fprintf(out, "csk: downloading %s (%d bytes)...\n", asset.Name, asset.Size)

			tmpDir, err := os.MkdirTemp("", "csk-upgrade-*")
			if err != nil {
				return envErr(err)
			}
			defer os.RemoveAll(tmpDir)

			archivePath, err := client.DownloadAndVerify(ctx, asset, sums, tmpDir)
			if err != nil {
				return envErr(err)
			}
			fmt.Fprintln(out, "csk: checksum verified")

			binName := "csk"
			if runtime.GOOS == "windows" {
				binName = "csk.exe"
			}
			newBinary := filepath.Join(tmpDir, binName)
			if err := selfupdate.ExtractBinary(archivePath, newBinary); err != nil {
				return envErr(err)
			}

			currentExe, err := os.Executable()
			if err != nil {
				return envErr(err)
			}
			// Resolve symlinks so we replace the actual binary, not a wrapper.
			if resolved, err := filepath.EvalSymlinks(currentExe); err == nil {
				currentExe = resolved
			}

			fmt.Fprintf(out, "csk: installing to %s\n", currentExe)
			if err := selfupdate.SwapBinary(currentExe, newBinary); err != nil {
				return envErr(err)
			}
			fmt.Fprintf(out, "csk: upgraded to %s\n", latest)
			return nil
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only check for a newer release; do not install")
	cmd.Flags().BoolVar(&force, "force", false, "reinstall even if the latest is already the current version")
	return cmd
}
