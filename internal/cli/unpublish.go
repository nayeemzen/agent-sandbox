package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func newUnpublishCmd(opts *GlobalOptions) *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:           "unpublish [sandbox] [host-port|device]",
		Aliases:       []string{"unexpose"},
		Short:         "Remove published ports from a sandbox",
		Args:          cobra.MaximumNArgs(2),
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

			var sandbox string
			var target string

			switch len(args) {
			case 0:
				sandbox, err = chooseSandboxArg(s, nil, "sandbox", "Select sandbox to unpublish ports from", nil)
				if err != nil {
					return err
				}
				if !all {
					target, err = promptRequiredValue("target", "Host port or device name", "", "Provide a host port (e.g. 8080) or a device name (sandbox-port-...)")
					if err != nil {
						return err
					}
				}
			case 1:
				sandbox = strings.TrimSpace(args[0])
				if !all {
					target, err = promptRequiredValue("target", "Host port or device name", "", "Provide a host port (e.g. 8080) or a device name (sandbox-port-...)")
					if err != nil {
						return err
					}
				}
			default:
				sandbox = strings.TrimSpace(args[0])
				target = strings.TrimSpace(args[1])
			}

			if sandbox == "" {
				return newCLIError("missing required argument: sandbox", "Usage: sandbox unpublish <sandbox> <host-port|device>")
			}

			ports, err := incus.ListPublishedPorts(s, sandbox)
			if err != nil {
				return err
			}

			devices := []string{}
			if all {
				for _, p := range ports {
					devices = append(devices, p.Device)
				}
			} else {
				dev, err := resolveUnpublishTarget(ports, target)
				if err != nil {
					return err
				}
				devices = append(devices, dev)
			}

			for _, dev := range devices {
				if err := incus.UnpublishPort(ctx, s, sandbox, dev); err != nil {
					return err
				}
			}

			if opts.JSON {
				return writeJSON(cmd.OutOrStdout(), map[string]any{"sandbox": sandbox, "removed": len(devices)})
			}
			if all {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "unpublished %d ports from %s\n", len(devices), sandbox)
				return nil
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "unpublished %s from %s\n", devices[0], sandbox)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Remove all published ports for the sandbox")
	return cmd
}

func resolveUnpublishTarget(ports []incus.PublishedPort, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("host port or device name is required")
	}
	if strings.HasPrefix(target, incus.PortDevicePrefix) {
		for _, p := range ports {
			if p.Device == target {
				return p.Device, nil
			}
		}
		return "", fmt.Errorf("published port device %q not found", target)
	}

	if n, err := strconv.Atoi(target); err == nil {
		for _, p := range ports {
			if p.HostPort == n {
				return p.Device, nil
			}
		}
		return "", fmt.Errorf("no published port found for host port %d", n)
	}

	return "", fmt.Errorf("invalid target %q (expected host port number or device name)", target)
}
