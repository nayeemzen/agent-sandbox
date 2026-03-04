package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func newRenameCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "rename [old-name] [new-name]",
		Short:         "Rename a sandbox",
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

			oldName, newName, err := resolveRenameArgs(s, args)
			if err != nil {
				return err
			}

			if oldName == newName {
				if opts.JSON {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"from": oldName, "to": newName, "renamed": false, "status": "unchanged"})
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s already named %s\n", oldName, newName)
				return nil
			}

			sb, err := incus.GetSandbox(s, oldName)
			if err != nil {
				return decorateRenameSandboxError(err, oldName, newName)
			}
			if !sb.Managed {
				return fmt.Errorf("%q is not a sandbox-managed instance", oldName)
			}

			errOut := cmd.ErrOrStderr()
			showProgress := !opts.JSON && isTTY(errOut)
			if err := withProgress(errOut, showProgress, fmt.Sprintf("Renaming sandbox %q to %q", oldName, newName), func() error {
				return incus.RenameSandbox(ctx, s, oldName, newName)
			}); err != nil {
				return decorateRenameSandboxError(err, oldName, newName)
			}

			var renamed *incus.Sandbox
			if sbNew, getErr := incus.GetSandbox(s, newName); getErr == nil {
				renamed = &sbNew
			}
			if err := renameSandboxState(opts, oldName, newName, renamed); err != nil {
				return err
			}

			if opts.JSON {
				payload := map[string]any{
					"from":    oldName,
					"to":      newName,
					"renamed": true,
				}
				if renamed != nil {
					payload["status"] = renamed.Status
				}
				return writeJSON(cmd.OutOrStdout(), payload)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s renamed to %s\n", oldName, newName)
			return nil
		},
	}

	return cmd
}

func resolveRenameArgs(sandboxServer incusclient.InstanceServer, args []string) (string, string, error) {
	var oldName string
	var newName string
	var err error

	switch len(args) {
	case 0:
		oldName, err = chooseSandboxArg(sandboxServer, nil, "sandbox", "Select sandbox to rename", nil)
		if err != nil {
			return "", "", err
		}
		newName, err = promptRequiredValue("new-name", "New sandbox name", "", "Provide the target sandbox name as the second argument")
		if err != nil {
			return "", "", err
		}
	case 1:
		oldName = strings.TrimSpace(args[0])
		newName, err = promptRequiredValue("new-name", "New sandbox name", "", "Provide the target sandbox name as the second argument")
		if err != nil {
			return "", "", err
		}
	case 2:
		oldName = strings.TrimSpace(args[0])
		newName = strings.TrimSpace(args[1])
	default:
		return "", "", newCLIError("too many arguments (expected at most 2)", "Usage: sandbox rename <old-name> <new-name>")
	}

	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)

	if oldName == "" {
		return "", "", newCLIError("missing required argument: old-name", "Usage: sandbox rename <old-name> <new-name>")
	}
	if newName == "" {
		return "", "", newCLIError("missing required argument: new-name", "Usage: sandbox rename <old-name> <new-name>")
	}

	return oldName, newName, nil
}

func renameSandboxState(opts *GlobalOptions, oldName string, newName string, sb *incus.Sandbox) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}

	st, stPath, err := loadState(opts)
	if err != nil {
		return err
	}

	dst := state.Sandbox{Name: newName}
	if cur, ok := st.Sandboxes[oldName]; ok {
		dst = cur
		dst.Name = newName
	}
	if sb != nil {
		if strings.TrimSpace(sb.Name) != "" {
			dst.Name = strings.TrimSpace(sb.Name)
		}
		if strings.TrimSpace(sb.Template) != "" {
			dst.Template = strings.TrimSpace(sb.Template)
		}
		dst.CreatedAt = nonZeroTime(sb.CreatedAt, dst.CreatedAt)
		if strings.TrimSpace(sb.Status) != "" {
			dst.LastState = strings.TrimSpace(sb.Status)
		}
	}

	delete(st.Sandboxes, oldName)
	st.Sandboxes[newName] = dst

	moved := map[string]state.ManagedProc{}
	for procName, proc := range st.Procs[oldName] {
		if strings.TrimSpace(proc.Name) == "" {
			proc.Name = procName
		}
		proc.Sandbox = newName
		moved[procName] = proc
	}

	delete(st.Procs, oldName)
	delete(st.Procs, newName)
	if len(moved) > 0 {
		st.Procs[newName] = moved
	}

	return saveState(stPath, st)
}

func decorateRenameSandboxError(err error, oldName string, newName string) error {
	if err == nil {
		return nil
	}

	if incus.IsNotFound(err) {
		return newCLIError(
			fmt.Sprintf("sandbox %q not found", oldName),
			"List sandboxes with `sandbox ls --all` and try again",
		)
	}

	var existsErr *incus.SandboxExistsError
	if errors.As(err, &existsErr) {
		target := strings.TrimSpace(existsErr.Name)
		if target == "" {
			target = newName
		}
		if existsErr.Managed {
			status := strings.TrimSpace(existsErr.Status)
			if status == "" {
				status = "Unknown"
			}
			return newCLIError(
				fmt.Sprintf("sandbox %q already exists (state=%s)", target, status),
				fmt.Sprintf("Choose a different target name, or remove it first: sandbox delete %s --yes", target),
			)
		}

		return newCLIError(
			fmt.Sprintf("instance %q already exists and is not sandbox-managed", target),
			fmt.Sprintf("Choose another name, or manage/remove that instance with Incus (`incus list`, `incus delete %s`).", target),
		)
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "renaming of running instance not allowed") {
		return newCLIError(
			fmt.Sprintf("cannot rename running sandbox %q", oldName),
			fmt.Sprintf("Stop it first (`sandbox stop %s`) and retry `sandbox rename %s %s`.", oldName, oldName, newName),
		)
	}

	return err
}
