package cli

import "github.com/spf13/cobra"

func newKillCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("kill <sandbox> <proc>", "Stop a managed process in a sandbox")
}
