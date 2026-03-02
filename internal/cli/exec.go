package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	incusclient "github.com/lxc/incus/v6/client"
	"golang.org/x/term"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func newExecCmd(opts *GlobalOptions) *cobra.Command {
	var detach bool
	var procName string

	cmd := &cobra.Command{
		Use:           "exec <sandbox> [-- <command...>]",
		Short:         "Run a command inside a sandbox (foreground or detached)",
		Args:          cobra.ArbitraryArgs,
		SilenceUsage:  true,
		SilenceErrors: true, // exec needs to return the guest exit code without Cobra printing "Error: ..."
		RunE: func(cmd *cobra.Command, args []string) error {
			baseCtx := cmd.Context()
			if baseCtx == nil {
				baseCtx = context.Background()
			}
			ctx, stop := signal.NotifyContext(baseCtx, os.Interrupt, syscall.SIGTERM)
			defer stop()

			errw := cmd.ErrOrStderr()

			s, err := connectIncus(ctx, opts)
			if err != nil {
				HandleError(errw, err)
				return ExitCodeError{Code: 1}
			}

			var sandboxName string
			var guestCmd []string
			if len(args) == 0 {
				sandboxes, err := incus.ListSandboxes(s, false)
				if err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}

				sandboxName, err = pickRequiredArg("sandbox", "Select sandbox for exec", sandboxOptionsFromIncus(sandboxes))
				if err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}
			} else {
				sandboxName, guestCmd, err = parseExecArgs(cmd, args)
				if err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}
			}

			sb, err := incus.GetSandbox(s, sandboxName)
			if err != nil {
				HandleError(errw, err)
				return ExitCodeError{Code: 1}
			}
			if !sb.Managed {
				renderError(errw, fmt.Sprintf("%q is not a sandbox-managed instance", sandboxName), "")
				return ExitCodeError{Code: 1}
			}
			if !strings.EqualFold(sb.Status, "running") {
				renderError(errw, fmt.Sprintf("%q is not running (state=%s)", sandboxName, sb.Status), "")
				return ExitCodeError{Code: 1}
			}

			if detach {
				if procName == "" {
					renderError(errw, "--name is required with --detach", "Usage: sandbox exec <sandbox> --detach --name <proc> -- <cmd>")
					return ExitCodeError{Code: 1}
				}
				if err := validateProcName(procName); err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}
				if len(guestCmd) == 0 {
					renderError(errw, "a command is required with --detach", "Usage: sandbox exec <sandbox> --detach --name <proc> -- <cmd>")
					return ExitCodeError{Code: 1}
				}

				pid, err := startDetachedProc(ctx, s, sandboxName, procName, guestCmd)
				if err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}

				st, stPath, err := loadState(opts)
				if err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}

				proc := state.ManagedProc{
					Sandbox:   sandboxName,
					Name:      procName,
					Command:   guestCmd,
					PID:       pid,
					LogPath:   managedProcLogPath(procName),
					PidPath:   managedProcPidPath(procName),
					StartedAt: time.Now().UTC(),
					Status:    state.ProcRunning,
				}
				upsertManagedProc(&st, proc)
				if err := saveState(stPath, st); err != nil {
					HandleError(errw, err)
					return ExitCodeError{Code: 1}
				}

				if opts.JSON {
					_ = writeJSON(cmd.OutOrStdout(), proc)
					return nil
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s started (pid=%d log=%s)\n", procName, pid, proc.LogPath)
				return nil
			}

			execCmd := guestCmd
			execOpts := incus.ExecOptions{
				Interactive: false,
				Stdin:       cmd.InOrStdin(),
				Stdout:      cmd.OutOrStdout(),
				Stderr:      cmd.ErrOrStderr(),
			}

			// With no command, open an interactive shell.
			if len(execCmd) == 0 {
				execCmd = []string{"sh"}
				execOpts.Interactive = true

				// Interactive exec needs raw terminal mode to avoid
				// terminal control sequences leaking into shell input.
				if stdinFile, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(stdinFile.Fd())) {
					oldState, err := term.MakeRaw(int(stdinFile.Fd()))
					if err != nil {
						renderError(errw, fmt.Sprintf("failed to enter raw terminal mode: %v", err), "")
						return ExitCodeError{Code: 1}
					}
					defer func() {
						_ = term.Restore(int(stdinFile.Fd()), oldState)
					}()
				}
			}

			res, err := incus.Exec(ctx, s, sandboxName, execCmd, execOpts)
			if err != nil {
				// Context cancellation is typically a user interrupt.
				if errors.Is(err, context.Canceled) {
					return ExitCodeError{Code: 130}
				}
				HandleError(errw, err)
				return ExitCodeError{Code: 1}
			}

			if res.ExitCode != 0 {
				return ExitCodeError{Code: res.ExitCode}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&detach, "detach", false, "Run the command in the background as a managed process")
	cmd.Flags().StringVar(&procName, "name", "", "Managed process name (required with --detach)")

	return cmd
}

func parseExecArgs(cmd *cobra.Command, args []string) (sandbox string, guestCmd []string, _ error) {
	if len(args) < 1 {
		return "", nil, fmt.Errorf("sandbox name is required")
	}

	dash := cmd.ArgsLenAtDash()
	if dash != -1 && dash != 1 {
		return "", nil, fmt.Errorf("expected exactly one argument before --")
	}

	sandbox = args[0]
	if dash == -1 {
		if len(args) > 1 {
			guestCmd = args[1:]
		}
		return sandbox, guestCmd, nil
	}

	if len(args) > dash {
		guestCmd = args[dash:]
	}
	return sandbox, guestCmd, nil
}

func startDetachedProc(ctx context.Context, s incusclient.InstanceServer, sandbox string, procName string, guestCmd []string) (pid int, _ error) {
	logPath := managedProcLogPath(procName)
	pidPath := managedProcPidPath(procName)

	// Use `sh -c` with "$@" to avoid quoting/escaping user commands.
	// shellcheck disable=SC2016
	script := fmt.Sprintf(`set -eu
mkdir -p /var/log/sandbox /run/sandbox
log=%q
pidfile=%q
nohup "$@" </dev/null >"$log" 2>&1 &
pid=$!
echo "$pid" >"$pidfile"
echo "$pid"
`, logPath, pidPath)

	execCmd := append([]string{"sh", "-lc", script, "--"}, guestCmd...)

	var out bytes.Buffer
	var errBuf bytes.Buffer
	res, err := incus.Exec(ctx, s, sandbox, execCmd, incus.ExecOptions{Stdout: &out, Stderr: &errBuf})
	if err != nil {
		return 0, err
	}
	if res.ExitCode != 0 {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			msg = fmt.Sprintf("guest returned %d", res.ExitCode)
		}
		return 0, fmt.Errorf("failed to start managed proc: %s", msg)
	}

	pidStr := strings.TrimSpace(out.String())
	if pidStr == "" {
		return 0, fmt.Errorf("failed to start managed proc: did not receive pid")
	}

	n, err := strconv.Atoi(pidStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("failed to start managed proc: invalid pid %q", pidStr)
	}

	return n, nil
}
