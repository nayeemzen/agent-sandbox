package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestDecorateIncusConnectError_LocalSocketPermissionDenied(t *testing.T) {
	t.Parallel()

	raw := errors.New(`Get "http://unix.socket/1.0": dial unix /var/lib/incus/unix.socket: connect: permission denied`)
	opts := &GlobalOptions{IncusUnixSocket: "/var/lib/incus/unix.socket"}

	err := decorateIncusConnectError(raw, opts)

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T (%v)", err, err)
	}
	if ce.Message != raw.Error() {
		t.Fatalf("wrapped message mismatch:\n got: %q\nwant: %q", ce.Message, raw.Error())
	}
	if !strings.Contains(ce.Hint, "tmux kill-server") {
		t.Fatalf("expected tmux remediation hint, got: %q", ce.Hint)
	}
	if !strings.Contains(ce.Hint, "/var/lib/incus/unix.socket") {
		t.Fatalf("expected socket path in hint, got: %q", ce.Hint)
	}
}

func TestDecorateIncusConnectError_RemoteURLDoesNotWrap(t *testing.T) {
	t.Parallel()

	raw := errors.New(`Get "http://unix.socket/1.0": dial unix /var/lib/incus/unix.socket: connect: permission denied`)
	opts := &GlobalOptions{IncusRemoteURL: "https://incus.example:8443"}

	err := decorateIncusConnectError(raw, opts)
	if !errors.Is(err, raw) {
		t.Fatalf("expected raw error passthrough, got: %v", err)
	}
}

func TestDecorateIncusConnectError_NonPermissionErrorDoesNotWrap(t *testing.T) {
	t.Parallel()

	raw := errors.New("connection refused")
	opts := &GlobalOptions{IncusUnixSocket: "/var/lib/incus/unix.socket"}

	err := decorateIncusConnectError(raw, opts)
	if !errors.Is(err, raw) {
		t.Fatalf("expected raw error passthrough, got: %v", err)
	}
}
