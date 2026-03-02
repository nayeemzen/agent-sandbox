package cli

import "github.com/spf13/cobra"

func newLogsCmd() *cobra.Command {
	var proc string

	cmd := newNotImplementedCmd("logs <sandbox>", "Tail logs for a managed process in a sandbox")
	cmd.Flags().StringVar(&proc, "proc", "", "Managed process name")

	return cmd
}
