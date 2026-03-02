package cli

import "github.com/spf13/cobra"

func newNewCmd() *cobra.Command {
	var template string

	cmd := newNotImplementedCmd("new <name>", "Create a sandbox quickly from an existing template (fast path)")
	cmd.Flags().StringVar(&template, "template", "", "Template name to use")

	return cmd
}
