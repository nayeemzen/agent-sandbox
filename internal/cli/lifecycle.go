package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func newPauseCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "pause <name>",
		Short:         "Pause a running sandbox",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sb, err := incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if !sb.Managed {
				return fmt.Errorf("%q is not a sandbox-managed instance", name)
			}

			if strings.EqualFold(sb.Status, "frozen") {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s already paused\n", name)
				return nil
			}

			if err := incus.PauseSandbox(ctx, s, name); err != nil {
				return err
			}

			sb, err = incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if err := upsertSandboxState(opts, sb); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s paused\n", name)
			return nil
		},
	}

	return cmd
}

func newResumeCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "resume <name>",
		Short:         "Resume a paused sandbox",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sb, err := incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if !sb.Managed {
				return fmt.Errorf("%q is not a sandbox-managed instance", name)
			}

			if !strings.EqualFold(sb.Status, "frozen") {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s is not paused (state=%s)\n", name, sb.Status)
				return nil
			}

			if err := incus.ResumeSandbox(ctx, s, name); err != nil {
				return err
			}

			sb, err = incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if err := upsertSandboxState(opts, sb); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s resumed\n", name)
			return nil
		},
	}

	return cmd
}

func newStopCmd(opts *GlobalOptions) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:           "stop <name>",
		Short:         "Stop a running sandbox",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sb, err := incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if !sb.Managed {
				return fmt.Errorf("%q is not a sandbox-managed instance", name)
			}

			if strings.EqualFold(sb.Status, "stopped") {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s already stopped\n", name)
				return nil
			}

			if err := incus.StopSandbox(ctx, s, name, force); err != nil {
				return err
			}

			sb, err = incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if err := upsertSandboxState(opts, sb); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s stopped\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force stop (may drop connections)")
	return cmd
}

func newStartCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "start <name>",
		Short:         "Start a stopped sandbox",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sb, err := incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if !sb.Managed {
				return fmt.Errorf("%q is not a sandbox-managed instance", name)
			}

			if strings.EqualFold(sb.Status, "running") {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s already running\n", name)
				return nil
			}

			// UX simplification: start treats a paused sandbox like resume.
			if strings.EqualFold(sb.Status, "frozen") {
				if err := incus.ResumeSandbox(ctx, s, name); err != nil {
					return err
				}
			} else {
				if err := incus.StartSandbox(ctx, s, name); err != nil {
					return err
				}
			}

			sb, err = incus.GetSandbox(s, name)
			if err != nil {
				return err
			}
			if err := upsertSandboxState(opts, sb); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s started\n", name)
			return nil
		},
	}

	return cmd
}

func newDeleteCmd(opts *GlobalOptions) *cobra.Command {
	var force bool
	var yes bool

	cmd := &cobra.Command{
		Use:           "delete <name>",
		Short:         "Delete a sandbox",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			if !force && !yes {
				return fmt.Errorf("refusing to delete %q without --yes (or use --force)", name)
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			// Protect non-sandbox instances from accidental deletion.
			if sb, err := incus.GetSandbox(s, name); err == nil {
				if !sb.Managed {
					return fmt.Errorf("%q is not a sandbox-managed instance", name)
				}
			}

			if err := incus.DeleteSandbox(ctx, s, name, force); err != nil {
				return err
			}

			if err := removeSandboxState(opts, name); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s deleted\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force delete (stop immediately, skip confirmation)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return cmd
}

func upsertSandboxState(opts *GlobalOptions, sb incus.Sandbox) error {
	st, stPath, err := loadState(opts)
	if err != nil {
		return err
	}

	st.Sandboxes[sb.Name] = state.Sandbox{
		Name:      sb.Name,
		Template:  sb.Template,
		CreatedAt: nonZeroTime(sb.CreatedAt, time.Now().UTC()),
		LastState: sb.Status,
	}

	return saveState(stPath, st)
}

func removeSandboxState(opts *GlobalOptions, name string) error {
	st, stPath, err := loadState(opts)
	if err != nil {
		return err
	}

	delete(st.Sandboxes, name)
	delete(st.Procs, name)
	return saveState(stPath, st)
}
