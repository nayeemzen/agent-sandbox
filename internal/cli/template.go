package cli

import "github.com/spf13/cobra"

func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage templates (immutable bases used by sandbox new)",
	}

	cmd.AddCommand(
		newNotImplementedCmd("add <name> <source>", "Create a template from a source (slow path)"),
		newNotImplementedCmd("ls", "List templates"),
		newTemplateRmCmd(),
		newNotImplementedCmd("default <name>", "Set the default template used by sandbox new"),
	)

	return cmd
}

func newTemplateRmCmd() *cobra.Command {
	var all bool

	cmd := newNotImplementedCmd("rm <name>", "Remove a template and its backend artifact")
	cmd.Flags().BoolVar(&all, "all", false, "Remove all templates")

	return cmd
}
