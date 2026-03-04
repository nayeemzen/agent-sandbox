package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func TestDecorateNewSandboxCreateError_ManagedStopped(t *testing.T) {
	t.Parallel()

	err := decorateNewSandboxCreateError(&incus.SandboxExistsError{
		Name:    "openclaw",
		Managed: true,
		Status:  "Stopped",
	}, "openclaw")

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if !strings.Contains(ce.Message, `sandbox "openclaw" already exists`) {
		t.Fatalf("unexpected message: %q", ce.Message)
	}
	if !strings.Contains(ce.Hint, "sandbox start openclaw") {
		t.Fatalf("expected start hint, got: %q", ce.Hint)
	}
}

func TestDecorateNewSandboxCreateError_ManagedRunning(t *testing.T) {
	t.Parallel()

	err := decorateNewSandboxCreateError(&incus.SandboxExistsError{
		Name:    "openclaw",
		Managed: true,
		Status:  "Running",
	}, "openclaw")

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if !strings.Contains(ce.Hint, "sandbox exec openclaw") {
		t.Fatalf("expected exec hint, got: %q", ce.Hint)
	}
}

func TestDecorateNewSandboxCreateError_Unmanaged(t *testing.T) {
	t.Parallel()

	err := decorateNewSandboxCreateError(&incus.SandboxExistsError{
		Name:    "openclaw",
		Managed: false,
		Status:  "Running",
	}, "openclaw")

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if !strings.Contains(ce.Message, "not sandbox-managed") {
		t.Fatalf("unexpected message: %q", ce.Message)
	}
	if !strings.Contains(ce.Hint, "incus delete openclaw") {
		t.Fatalf("expected incus hint, got: %q", ce.Hint)
	}
}

func TestDecorateNewSandboxCreateError_PassThrough(t *testing.T) {
	t.Parallel()

	raw := errors.New("something else")
	err := decorateNewSandboxCreateError(raw, "openclaw")
	if !errors.Is(err, raw) {
		t.Fatalf("expected passthrough, got: %v", err)
	}
}
