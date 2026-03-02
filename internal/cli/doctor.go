package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/doctor"
	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func newDoctorCmd() *cobra.Command {
	var remoteURL string
	var unixSocket string
	var insecure bool

	cmd := &cobra.Command{
		Use:           "doctor",
		Short:         "Run diagnostics to check whether the environment is ready",
		SilenceUsage:  true,
		SilenceErrors: true, // doctor prints its own results
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			s, err := incus.Connect(ctx, incus.ConnectOptions{
				UnixSocket:         unixSocket,
				RemoteURL:          remoteURL,
				InsecureSkipVerify: insecure,
			})
			if err != nil {
				res := []doctor.CheckResult{
					{
						ID:          "incus.connect",
						Status:      doctor.Fail,
						Summary:     "failed to connect to Incus",
						Details:     err.Error(),
						Remediation: remediationForConnectError(err),
					},
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), doctor.RenderHuman(res))
				return fmt.Errorf("doctor failed")
			}

			results := incus.RunDoctor(ctx, s, incus.DoctorOptions{
				LocalMode: remoteURL == "",
			})

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), doctor.RenderHuman(results))
			if doctor.ExitCode(results) != 0 {
				return fmt.Errorf("doctor failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&remoteURL, "remote-url", "", "Remote Incus HTTPS URL (for example https://host:8443)")
	cmd.Flags().StringVar(&unixSocket, "unix-socket", "/var/lib/incus/unix.socket", "Local Incus unix socket path")
	cmd.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS verification for --remote-url (debug only)")

	return cmd
}

func remediationForConnectError(err error) string {
	msg := err.Error()

	if strings.Contains(msg, "permission") || strings.Contains(msg, "needed permissions") {
		return "Add your user to the incus-admin group (or incus group for restricted access), then log out/in and retry."
	}

	if strings.Contains(msg, "No such file or directory") && strings.Contains(msg, "unix") {
		return "Is Incus installed and initialized? Try: sudo incus admin init --minimal"
	}

	return "Verify Incus is installed, initialized, and reachable (local socket or remote URL)."
}
