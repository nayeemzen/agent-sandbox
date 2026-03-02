package cli

import (
	"time"

	"github.com/spf13/cobra"
)

type GlobalOptions struct {
	JSON bool

	ConfigPath string
	StatePath  string

	IncusUnixSocket string
	IncusRemoteURL  string
	IncusProject    string
	IncusInsecure   bool
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
	cmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "", "Config file path (default: XDG config dir)")
	cmd.PersistentFlags().StringVar(&opts.StatePath, "state", "", "State file path (default: XDG state dir)")
	cmd.PersistentFlags().StringVar(&opts.IncusUnixSocket, "incus-unix-socket", "/var/lib/incus/unix.socket", "Incus unix socket path")
	cmd.PersistentFlags().StringVar(&opts.IncusRemoteURL, "incus-remote-url", "", "Incus remote HTTPS URL (for example https://host:8443)")
	cmd.PersistentFlags().StringVar(&opts.IncusProject, "incus-project", "default", "Incus project name")
	cmd.PersistentFlags().BoolVar(&opts.IncusInsecure, "incus-insecure", false, "Skip TLS verification for --incus-remote-url (debug only)")

	cmd.AddCommand(
		newInstallCmd(opts),
		newSetupCmd(opts),
		newDoctorCmd(opts),
		newInitCmd(opts),
		newTemplateCmd(opts),
		newNewCmd(opts),
		newLsCmd(opts),
		newExecCmd(opts),
		newLogsCmd(opts),
		newPsCmd(opts),
		newKillCmd(opts),
		newPauseCmd(opts),
		newResumeCmd(opts),
		newStopCmd(opts),
		newStartCmd(opts),
		newDeleteCmd(opts),
		newMonitorCmd(opts, time.Second*15),
		newCompletionCmd(),
	)

	return cmd
}
