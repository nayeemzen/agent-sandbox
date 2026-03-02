package cli

import "github.com/spf13/cobra"

func newExecCmd() *cobra.Command {
	var detach bool
	var procName string

	cmd := newNotImplementedCmd("exec <sandbox> [-- <command...>]", "Run a command inside a sandbox (foreground or detached)")
	cmd.Flags().BoolVar(&detach, "detach", false, "Run the command in the background as a managed process")
	cmd.Flags().StringVar(&procName, "name", "", "Managed process name (required with --detach)")

	return cmd
}
