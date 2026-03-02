package cli

import "github.com/spf13/cobra"

func newPsCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("ps <sandbox>", "List managed processes started by sandbox exec --detach")
}
