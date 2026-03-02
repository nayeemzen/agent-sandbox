package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func newLsCmd(opts *GlobalOptions) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:           "ls",
		Short:         "List sandboxes",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sandboxes, err := incus.ListSandboxes(s, all)
			if err != nil {
				return err
			}

			sort.Slice(sandboxes, func(i, j int) bool { return sandboxes[i].Name < sandboxes[j].Name })

			if opts.JSON {
				return writeJSON(cmd.OutOrStdout(), sandboxes)
			}

			if len(sandboxes) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no sandboxes)")
				return nil
			}

			// Simple stable output (v1): NAME STATE AGE IPS TEMPLATE
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "NAME\tSTATE\tAGE\tIPS\tTEMPLATE")
			now := time.Now()
			for _, sb := range sandboxes {
				age := ""
				if !sb.CreatedAt.IsZero() {
					age = humanDuration(now.Sub(sb.CreatedAt))
				}

				ips := "-"
				if len(sb.IPv4)+len(sb.IPv6) > 0 {
					ips = strings.Join(append(append([]string{}, sb.IPv4...), sb.IPv6...), ",")
				}

				tpl := strings.TrimSpace(sb.Template)
				if tpl == "" {
					tpl = "-"
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", sb.Name, sb.Status, age, ips, tpl)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Include stopped/paused sandboxes")

	return cmd
}
