package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newMonitorCmd(defaultInterval time.Duration) *cobra.Command {
	var interval time.Duration

	cmd := newNotImplementedCmd("monitor", "Show live per-sandbox resource usage relative to host capacity")
	cmd.Flags().DurationVar(&interval, "interval", defaultInterval, "Polling interval")

	return cmd
}
