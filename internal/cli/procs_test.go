package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func TestValidateProcName(t *testing.T) {
	t.Parallel()

	ok := []string{"web", "web-1", "web_1", "web.1", "a", "A0", "a-b_c.d"}
	for _, name := range ok {
		name := name
		t.Run("ok_"+name, func(t *testing.T) {
			t.Parallel()
			if err := validateProcName(name); err != nil {
				t.Fatalf("expected %q to be valid, got error: %v", name, err)
			}
		})
	}

	bad := []string{"", " ", "a/b", "../x", "-nope", "nope!", "x x", "a\nb"}
	for _, name := range bad {
		name := name
		t.Run("bad_"+name, func(t *testing.T) {
			t.Parallel()
			if err := validateProcName(name); err == nil {
				t.Fatalf("expected %q to be invalid", name)
			}
		})
	}
}

func TestUpsertManagedProc_CreatesNestedMaps(t *testing.T) {
	t.Parallel()

	st := state.Default()
	st.Procs = nil

	p := state.ManagedProc{
		Sandbox:   "sb1",
		Name:      "web",
		Command:   []string{"python", "-m", "http.server", "8000"},
		PID:       123,
		LogPath:   "/var/log/sandbox/web.log",
		PidPath:   "/run/sandbox/web.pid",
		StartedAt: time.Now().UTC(),
		Status:    state.ProcRunning,
	}

	upsertManagedProc(&st, p)

	if st.Procs == nil {
		t.Fatalf("expected Procs to be initialized")
	}
	if st.Procs["sb1"] == nil {
		t.Fatalf("expected Procs[sb1] to be initialized")
	}
	if got := st.Procs["sb1"]["web"]; got.PID != 123 || got.LogPath == "" {
		t.Fatalf("unexpected stored proc: %#v", got)
	}
}

func TestSelectProcForLogs(t *testing.T) {
	t.Parallel()

	t.Run("explicit_found", func(t *testing.T) {
		t.Parallel()
		procs := map[string]state.ManagedProc{
			"web": {Sandbox: "sb1", Name: "web"},
		}
		got, err := selectProcForLogs("sb1", procs, "web")
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if got.Name != "web" {
			t.Fatalf("got name=%q want web", got.Name)
		}
	})

	t.Run("explicit_missing_with_candidates", func(t *testing.T) {
		t.Parallel()
		procs := map[string]state.ManagedProc{
			"a": {Sandbox: "sb1", Name: "a"},
			"b": {Sandbox: "sb1", Name: "b"},
		}
		_, err := selectProcForLogs("sb1", procs, "c")
		if err == nil {
			t.Fatalf("expected error")
		}
		if want := `process "c" not found in "sb1"`; !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	})

	t.Run("implicit_none", func(t *testing.T) {
		t.Parallel()
		_, err := selectProcForLogs("sb1", nil, "")
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("implicit_one", func(t *testing.T) {
		t.Parallel()
		procs := map[string]state.ManagedProc{
			"web": {Sandbox: "sb1", Name: "web"},
		}
		got, err := selectProcForLogs("sb1", procs, "")
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
		if got.Name != "web" {
			t.Fatalf("got name=%q want web", got.Name)
		}
	})

	t.Run("implicit_multiple", func(t *testing.T) {
		t.Parallel()
		procs := map[string]state.ManagedProc{
			"a": {Sandbox: "sb1", Name: "a"},
			"b": {Sandbox: "sb1", Name: "b"},
		}
		_, err := selectProcForLogs("sb1", procs, "")
		if err == nil {
			t.Fatalf("expected error")
		}
		if want := `multiple processes found in "sb1"`; !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	})
}
