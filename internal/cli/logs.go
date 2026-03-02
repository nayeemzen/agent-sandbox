package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func newLogsCmd(opts *GlobalOptions) *cobra.Command {
	var proc string

	cmd := &cobra.Command{
		Use:           "logs [sandbox]",
		Short:         "Tail logs for a managed process or a /var/log/sandbox file in a sandbox",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseCtx := cmd.Context()
			if baseCtx == nil {
				baseCtx = context.Background()
			}
			ctx, stop := signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			sandbox, err := chooseSandboxArg(s, args, "sandbox", "Select sandbox for logs", func(sb incus.Sandbox) bool {
				return strings.EqualFold(sb.Status, "running")
			})
			if err != nil {
				return err
			}

			st, _, err := loadState(opts)
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

			var logPath string
			var managedProcName string
			if proc != "" {
				managedProcName = proc
			} else {
				candidates := procOptionsFromState(st, sandbox)
				switch len(candidates) {
				case 0:
					logPath, err = pickLogPathForSandbox(ctx, s, sandbox)
					if err != nil {
						return err
					}
				case 1:
					managedProcName = candidates[0].Value
				default:
					managedProcName, err = pickRequiredArg("proc", "Select managed process for logs", candidates)
					if err != nil {
						return err
					}
				}
			}

			if logPath == "" {
				if managedProcName != "" {
					// Allow direct --proc use even when state metadata is missing.
					if p, ok := st.Procs[sandbox][managedProcName]; ok {
						logPath = p.LogPath
					}
					if strings.TrimSpace(logPath) == "" {
						logPath = managedProcLogPath(managedProcName)
					}
				} else {
					managed, err := selectProcForLogs(sandbox, st.Procs[sandbox], managedProcName)
					if err != nil {
						return err
					}
					logPath = managed.LogPath
					if logPath == "" {
						logPath = managedProcLogPath(managed.Name)
					}
				}
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

func pickLogPathForSandbox(ctx context.Context, s incusclient.InstanceServer, sandbox string) (string, error) {
	logFiles, err := listSandboxLogFiles(ctx, s, sandbox)
	if err != nil {
		return "", err
	}

	if len(logFiles) == 0 {
		return "", newCLIError(
			fmt.Sprintf("no logs found for %q", sandbox),
			fmt.Sprintf("Start a background process first:\n  sandbox exec %s --detach --name <proc> -- <cmd>", sandbox),
		)
	}

	if len(logFiles) == 1 {
		return logFiles[0], nil
	}

	options := make([]selectOption, 0, len(logFiles))
	for _, path := range logFiles {
		options = append(options, selectOption{Label: path, Value: path})
	}
	return pickRequiredArg("logfile", "Select log file", options)
}

func listSandboxLogFiles(ctx context.Context, s incusclient.InstanceServer, sandbox string) ([]string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Portable shell loop (works without GNU find -printf).
	script := `d=/var/log/sandbox
if [ ! -d "$d" ]; then
  exit 0
fi
for f in "$d"/*.log; do
  [ -f "$f" ] || continue
  printf '%s\n' "$f"
done`

	res, err := incus.Exec(ctx, s, sandbox, []string{"sh", "-lc", script}, incus.ExecOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("failed to enumerate logs in %q (exit=%d): %s", sandbox, res.ExitCode, msg)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	sort.Strings(out)
	return out, nil
}
