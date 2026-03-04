package cli

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func TestRenameSandboxState_MovesSandboxAndProcs(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	opts := &GlobalOptions{
		StatePath: filepath.Join(tmp, "state.json"),
	}

	started := time.Unix(10, 0).UTC()
	st := state.Default()
	st.Sandboxes["old"] = state.Sandbox{
		Name:      "old",
		Template:  "base",
		CreatedAt: started,
		LastState: "Running",
	}
	st.Procs["old"] = map[string]state.ManagedProc{
		"web": {
			Sandbox:   "old",
			Name:      "web",
			PID:       1234,
			LogPath:   "/var/log/sandbox/web.log",
			PidPath:   "/run/sandbox/web.pid",
			StartedAt: started,
			Status:    state.ProcRunning,
		},
	}
	if err := saveState(opts.StatePath, st); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	after := &incus.Sandbox{
		Name:      "new",
		Template:  "base",
		Status:    "Stopped",
		CreatedAt: time.Unix(20, 0).UTC(),
	}
	if err := renameSandboxState(opts, "old", "new", after); err != nil {
		t.Fatalf("renameSandboxState: %v", err)
	}

	got, _, err := loadState(opts)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if _, ok := got.Sandboxes["old"]; ok {
		t.Fatalf("old sandbox key should be removed")
	}
	sb, ok := got.Sandboxes["new"]
	if !ok {
		t.Fatalf("new sandbox key missing")
	}
	if sb.Name != "new" {
		t.Fatalf("sandbox name = %q, want new", sb.Name)
	}
	if sb.LastState != "Stopped" {
		t.Fatalf("sandbox last state = %q, want Stopped", sb.LastState)
	}
	if !sb.CreatedAt.Equal(after.CreatedAt) {
		t.Fatalf("sandbox created_at = %v, want %v", sb.CreatedAt, after.CreatedAt)
	}

	if _, ok := got.Procs["old"]; ok {
		t.Fatalf("old procs key should be removed")
	}
	pm, ok := got.Procs["new"]
	if !ok {
		t.Fatalf("new procs key missing")
	}
	proc, ok := pm["web"]
	if !ok {
		t.Fatalf("proc web missing after rename")
	}
	if proc.Sandbox != "new" {
		t.Fatalf("proc sandbox = %q, want new", proc.Sandbox)
	}
}

func TestDecorateRenameSandboxError_ExistsManaged(t *testing.T) {
	t.Parallel()

	err := decorateRenameSandboxError(&incus.SandboxExistsError{
		Name:    "target",
		Managed: true,
		Status:  "Running",
	}, "old", "target")

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if !strings.Contains(ce.Message, `sandbox "target" already exists`) {
		t.Fatalf("unexpected message: %q", ce.Message)
	}
}

func TestDecorateRenameSandboxError_ExistsUnmanaged(t *testing.T) {
	t.Parallel()

	err := decorateRenameSandboxError(&incus.SandboxExistsError{
		Name:    "target",
		Managed: false,
		Status:  "Running",
	}, "old", "target")

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if !strings.Contains(ce.Message, "not sandbox-managed") {
		t.Fatalf("unexpected message: %q", ce.Message)
	}
}

func TestDecorateRenameSandboxError_RunningRenameNotAllowed(t *testing.T) {
	t.Parallel()

	err := decorateRenameSandboxError(errors.New("Renaming of running instance not allowed"), "old", "new")

	var ce *CLIError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if !strings.Contains(ce.Message, "cannot rename running sandbox") {
		t.Fatalf("unexpected message: %q", ce.Message)
	}
	if !strings.Contains(ce.Hint, "sandbox stop old") {
		t.Fatalf("unexpected hint: %q", ce.Hint)
	}
}
