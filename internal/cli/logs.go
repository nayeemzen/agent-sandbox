package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func newLogsCmd(opts *GlobalOptions) *cobra.Command {
	var proc string

	cmd := &cobra.Command{
		Use:           "logs <sandbox>",
		Short:         "Tail logs for a managed process in a sandbox",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseCtx := cmd.Context()
			if baseCtx == nil {
				baseCtx = context.Background()
			}
			ctx, stop := signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			sandbox := args[0]

			st, _, err := loadState(opts)
			if err != nil {
				return err
			}

			managed, err := selectProcForLogs(sandbox, st.Procs[sandbox], proc)
			if err != nil {
				return err
			}

			logPath := managed.LogPath
			if logPath == "" {
				logPath = managedProcLogPath(managed.Name)
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sb, err := incus.GetSandbox(s, sandbox)
			if err != nil {
				return err
			}
			if !sb.Managed {
				return fmt.Errorf("%q is not a sandbox-managed instance", sandbox)
			}

			// Use `sh -c` with an argument to avoid quoting the path.
			execCmd := []string{"sh", "-lc", "tail -n 200 -F \"$1\"", "--", logPath}
			res, err := incus.Exec(ctx, s, sandbox, execCmd, incus.ExecOptions{
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			})
			if err != nil {
				// Treat user interrupts as success.
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}

			if res.ExitCode != 0 {
				return fmt.Errorf("tail exited with %d", res.ExitCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&proc, "proc", "", "Managed process name")

	return cmd
}
