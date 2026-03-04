package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func newPortsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ports [sandbox]",
		Short:         "List published ports for a sandbox",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}
			sandbox, err := chooseSandboxArg(s, args, "sandbox", "Select sandbox to list ports for", nil)
			if err != nil {
				return err
			}

			ports, err := incus.ListPublishedPorts(s, sandbox)
			if err != nil {
				return err
			}
			sort.Slice(ports, func(i, j int) bool {
				if ports[i].HostPort != ports[j].HostPort {
					return ports[i].HostPort < ports[j].HostPort
				}
				return ports[i].Device < ports[j].Device
			})

			if opts.JSON {
				out := make([]publishPortJSON, 0, len(ports))
				for _, p := range ports {
					out = append(out, publishPortJSON{
						Sandbox:        sandbox,
						Device:         p.Device,
						Protocol:       p.Protocol,
						ListenAddress:  p.ListenAddress,
						HostPort:       p.HostPort,
						ConnectAddress: p.ConnectAddress,
						GuestPort:      p.GuestPort,
						Created:        false,
					})
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}

			if len(ports) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "(no published ports for %s)\n", sandbox)
				return nil
			}

			headers := []string{"PROTO", "LISTEN", "CONNECT", "DEVICE"}
			rows := make([][]string, 0, len(ports))
			for _, p := range ports {
				listen := fmt.Sprintf("%s:%d", p.ListenAddress, p.HostPort)
				connect := fmt.Sprintf("%s:%d", p.ConnectAddress, p.GuestPort)
				rows = append(rows, []string{strings.ToLower(p.Protocol), listen, connect, p.Device})
			}
			renderTable(cmd.OutOrStdout(), headers, rows)
			return nil
		},
	}

	return cmd
}
