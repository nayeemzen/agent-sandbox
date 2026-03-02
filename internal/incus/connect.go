package incus

import (
	"context"

	incusclient "github.com/lxc/incus/v6/client"
)

type ConnectOptions struct {
	// UnixSocket is the path to the Incus unix socket. If empty, the default
	// system daemon socket path is used.
	UnixSocket string

	// RemoteURL is an HTTPS URL to a remote Incus server (for example https://host:8443).
	// If set, it takes precedence over UnixSocket.
	RemoteURL string

	// InsecureSkipVerify disables TLS verification for RemoteURL connections.
	// This should only be used for debugging.
	InsecureSkipVerify bool
}

func Connect(ctx context.Context, opts ConnectOptions) (incusclient.InstanceServer, error) {
	if opts.RemoteURL != "" {
		args := &incusclient.ConnectionArgs{
			InsecureSkipVerify: opts.InsecureSkipVerify,
		}

		return incusclient.ConnectIncusWithContext(ctx, opts.RemoteURL, args)
	}

	socket := opts.UnixSocket
	if socket == "" {
		socket = "/var/lib/incus/unix.socket"
	}

	return incusclient.ConnectIncusUnixWithContext(ctx, socket, nil)
}
