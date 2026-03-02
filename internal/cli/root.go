package cli

import (
	"time"

	"github.com/spf13/cobra"
)

type GlobalOptions struct {
	JSON bool
}

func NewRootCmd() *cobra.Command {
	opts := &GlobalOptions{}

	cmd := &cobra.Command{
		Use:           "sandbox",
		Short:         "Incus-first sandboxes with a minimal CLI",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	cmd.PersistentFlags().BoolVar(&opts.JSON, "json", false, "Output machine-readable JSON")

	cmd.AddCommand(
		newNotImplementedCmd("setup", "Set up the local environment for running sandboxes"),
		newDoctorCmd(),
		newNotImplementedCmd("init", "Ensure a usable default template exists"),
		newTemplateCmd(),
		newNewCmd(),
		newLsCmd(),
		newExecCmd(),
		newLogsCmd(),
		newPsCmd(),
		newKillCmd(),
		newNotImplementedCmd("pause <name>", "Pause a running sandbox"),
		newNotImplementedCmd("resume <name>", "Resume a paused sandbox"),
		newNotImplementedCmd("stop <name>", "Stop a running sandbox"),
		newNotImplementedCmd("start <name>", "Start a stopped sandbox"),
		newNotImplementedCmd("delete <name>", "Delete a sandbox"),
		newMonitorCmd(time.Second*15),
	)

	return cmd
}
