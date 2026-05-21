package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/registry"
)

func newInfoCmd() *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:   "info <name>",
		Short: "Show the full registry entry for a skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			client := registry.New()
			idx, err := client.Fetch(ctx, refresh)
			if err != nil {
				return envErr(err)
			}
			s := idx.Find(name)
			if s == nil {
				return userErr(fmt.Errorf("skill %q not found in registry", name))
			}
			renderSkill(cmd.OutOrStdout(), s)
			return nil
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "force a fresh fetch from the registry")
	return cmd
}

func renderSkill(out interface{ Write([]byte) (int, error) }, s *registry.Skill) {
	display := s.DisplayName
	if display == "" {
		display = s.Name
	}
	lines := []string{
		fmt.Sprintf("%s (%s)", display, s.Name),
		fmt.Sprintf("  %s", s.Description),
		"",
		"  source     : " + s.Source,
	}
	if s.Subdir != "" {
		lines = append(lines, "  subdir     : "+s.Subdir)
	}
	if s.DefaultRef != "" {
		lines = append(lines, "  default ref: "+s.DefaultRef)
	}
	lines = append(lines,
		"  license    : "+s.License,
		"  maintainer : "+s.Maintainer,
	)
	if s.Homepage != "" {
		lines = append(lines, "  homepage   : "+s.Homepage)
	}
	if len(s.Categories) > 0 {
		lines = append(lines, "  categories : "+strings.Join(s.Categories, ", "))
	}
	if len(s.Tags) > 0 {
		lines = append(lines, "  tags       : "+strings.Join(s.Tags, ", "))
	}
	if s.Added != "" {
		lines = append(lines, "  added      : "+s.Added)
	}
	if s.Updated != "" {
		lines = append(lines, "  updated    : "+s.Updated)
	}
	lines = append(lines,
		"",
		fmt.Sprintf("install with:  csk add %s", s.Name),
	)
	_, _ = out.Write([]byte(strings.Join(lines, "\n") + "\n"))
}
