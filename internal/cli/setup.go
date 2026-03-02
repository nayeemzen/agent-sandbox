package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/doctor"
	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

type setupPlan struct {
	LocalMode bool
}

func computeSetupPlan(opts *GlobalOptions) setupPlan {
	return setupPlan{
		LocalMode: opts.IncusRemoteURL == "",
	}
}

func newSetupCmd(opts *GlobalOptions) *cobra.Command {
	var noInit bool

	cmd := &cobra.Command{
		Use:           "setup",
		Short:         "Set up the environment for running sandboxes",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			plan := computeSetupPlan(opts)

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			results := incus.RunDoctor(ctx, s, incus.DoctorOptions{LocalMode: plan.LocalMode})
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), doctor.RenderHuman(results))

			if doctor.ExitCode(results) != 0 {
				return fmt.Errorf("setup failed (fix failing checks and retry)")
			}

			if plan.LocalMode {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Note: if you use UFW, ensure DHCP is allowed on your Incus bridge (for example: ufw allow in on incusbr0 to any port 67 proto udp).")
			}

			if !noInit {
				if err := runInit(ctx, opts, cmd.OutOrStdout(), "images:ubuntu/24.04"); err != nil {
					return err
				}
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Skipping template init (--no-init). Run: sandbox init")
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "setup complete")
			return nil
		},
	}

	cmd.Flags().BoolVar(&noInit, "no-init", false, "Skip creating/selecting a default template")
	return cmd
}
