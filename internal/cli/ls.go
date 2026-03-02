package cli

import "github.com/spf13/cobra"

func newLsCmd(_ *GlobalOptions) *cobra.Command {
	var all bool

	cmd := newNotImplementedCmd("ls", "List sandboxes")
	cmd.Flags().BoolVar(&all, "all", false, "Include stopped/paused sandboxes")

	return cmd
}
