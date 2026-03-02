package cli

import "github.com/spf13/cobra"

func newSetupCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("setup", "Set up the local environment for running sandboxes")
}
