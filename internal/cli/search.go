package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/pformoso-deus-ai/csk/internal/registry"
)

func newSearchCmd() *cobra.Command {
	var (
		refresh bool
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search the public registry for skills",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) == 1 {
				query = args[0]
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			client := registry.New()
			idx, err := client.Fetch(ctx, refresh)
			if err != nil {
				return envErr(err)
			}
			results := idx.Search(query)
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "csk: no matches")
				return nil
			}
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tCATEGORIES\tDESCRIPTION")
			for _, r := range results {
				cats := strings.Join(r.Skill.Categories, ",")
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Skill.Name, cats, truncateMiddle(r.Skill.Description, 70))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "force a fresh fetch from the registry, bypassing the local cache")
	cmd.Flags().IntVar(&limit, "limit", 20, "max results to show (0 = unlimited)")
	return cmd
}

func truncateMiddle(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 4 {
		return s[:max]
	}
	return s[:max-1] + "…"
}
