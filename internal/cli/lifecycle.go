package cli

import "github.com/spf13/cobra"

func newPauseCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("pause <name>", "Pause a running sandbox")
}

func newResumeCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("resume <name>", "Resume a paused sandbox")
}

func newStopCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("stop <name>", "Stop a running sandbox")
}

func newStartCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("start <name>", "Start a stopped sandbox")
}

func newDeleteCmd(_ *GlobalOptions) *cobra.Command {
	return newNotImplementedCmd("delete <name>", "Delete a sandbox")
}
