package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func newKillCmd(opts *GlobalOptions) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:           "kill <sandbox> <proc>",
		Short:         "Stop a managed process in a sandbox",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			sandbox := args[0]
			procName := args[1]

			st, stPath, err := loadState(opts)
			if err != nil {
				return err
			}

			pmap := st.Procs[sandbox]
			if pmap == nil {
				return fmt.Errorf("no managed procs recorded for %q", sandbox)
			}
			p, ok := pmap[procName]
			if !ok {
				return fmt.Errorf("managed proc %q not found in %q", procName, sandbox)
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

			pidPath := p.PidPath
			if strings.TrimSpace(pidPath) == "" {
				pidPath = managedProcPidPath(procName)
			}

			// shellcheck disable=SC2016
			script := fmt.Sprintf(`set -eu
pidfile=%q
if [ ! -f "$pidfile" ]; then
  exit 0
fi
pid="$(cat "$pidfile" | tr -d '\n')"
if [ -z "$pid" ]; then
  rm -f "$pidfile"
  exit 0
fi
if kill -0 "$pid" 2>/dev/null; then
  kill "$pid" 2>/dev/null || true
  for _ in 1 2 3 4 5; do
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    sleep 0.2
  done
  if kill -0 "$pid" 2>/dev/null; then
    if %s; then
      kill -9 "$pid" 2>/dev/null || true
    else
      echo "process still running; re-run with --force to SIGKILL" 1>&2
      exit 1
    fi
  fi
fi
rm -f "$pidfile"
`, pidPath, boolToShell(force))

			res, err := incus.Exec(ctx, s, sandbox, []string{"sh", "-lc", script}, incus.ExecOptions{})
			if err != nil {
				return err
			}
			if res.ExitCode != 0 {
				return fmt.Errorf("kill failed (exit=%d)", res.ExitCode)
			}

			p.Status = state.ProcExited
			p.PID = 0
			pmap[procName] = p
			st.Procs[sandbox] = pmap
			if err := saveState(stPath, st); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s/%s stopped\n", sandbox, procName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force kill (SIGKILL if needed)")
	return cmd
}

func boolToShell(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
