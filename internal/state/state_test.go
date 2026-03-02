package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.json")
	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if st.Version == 0 {
		t.Fatalf("expected Version to be set")
	}
	if st.Sandboxes == nil {
		t.Fatalf("expected Sandboxes to be initialized")
	}
	if st.Procs == nil {
		t.Fatalf("expected Procs to be initialized")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "state.json")

	in := Default()
	in.Sandboxes["sb1"] = Sandbox{
		Name:      "sb1",
		Template:  "base",
		CreatedAt: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		LastState: "Running",
	}
	in.Procs["sb1"] = map[string]ManagedProc{
		"web": {
			Sandbox:   "sb1",
			Name:      "web",
			Command:   []string{"python", "-m", "http.server", "8000"},
			PID:       123,
			LogPath:   "/var/log/sandbox/web.log",
			PidPath:   "/run/sandbox/web.pid",
			StartedAt: time.Now().UTC(),
			Status:    ProcRunning,
		},
	}

	if err := Save(path, in); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected state file to exist: %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if out.Sandboxes["sb1"].Template != "base" {
		t.Fatalf("sandbox template = %q, want %q", out.Sandboxes["sb1"].Template, "base")
	}
	if out.Procs["sb1"]["web"].PID != 123 {
		t.Fatalf("proc pid = %d, want %d", out.Procs["sb1"]["web"].PID, 123)
	}
}

func TestSaveWritesTrailingNewline(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "state.json")
	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state file: %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("expected file to end with newline")
	}
}
