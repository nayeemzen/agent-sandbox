package cli

import (
	"fmt"
	"strings"
)

func decorateIncusConnectError(err error, opts *GlobalOptions) error {
	if err == nil {
		return nil
	}
	if opts == nil {
		return err
	}
	if strings.TrimSpace(opts.IncusRemoteURL) != "" {
		return err
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "permission denied") {
		return err
	}

	socket := strings.TrimSpace(opts.IncusUnixSocket)
	if socket == "" {
		socket = "/var/lib/incus/unix.socket"
	}
	socketLower := strings.ToLower(socket)

	// Limit this hint to local unix-socket access issues.
	if !strings.Contains(msg, "dial unix") &&
		!strings.Contains(msg, "unix.socket") &&
		!strings.Contains(msg, socketLower) {
		return err
	}

	hint := fmt.Sprintf(
		"Cannot access Incus socket %q. Ensure this shell has the incus-admin group. If tmux was started before group membership changed, restart tmux (`tmux kill-server`) or start a fresh tmux session, then open a new shell (or run `newgrp incus-admin`) and retry `sandbox doctor`.",
		socket,
	)
	return newCLIError(err.Error(), hint)
}
