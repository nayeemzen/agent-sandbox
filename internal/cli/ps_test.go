package cli

import (
	"testing"

	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func TestCollectManagedProcRows_AllSandboxesSorted(t *testing.T) {
	t.Parallel()

	st := state.Default()
	st.Procs["sb2"] = map[string]state.ManagedProc{
		"b": {},
		"a": {},
	}
	st.Procs["sb1"] = map[string]state.ManagedProc{
		"c": {},
	}

	rows := collectManagedProcRows(st, "")
	if len(rows) != 3 {
		t.Fatalf("len(rows)=%d, want 3", len(rows))
	}

	if rows[0].Sandbox != "sb1" || rows[0].Name != "c" {
		t.Fatalf("row[0]=%s/%s, want sb1/c", rows[0].Sandbox, rows[0].Name)
	}
	if rows[1].Sandbox != "sb2" || rows[1].Name != "a" {
		t.Fatalf("row[1]=%s/%s, want sb2/a", rows[1].Sandbox, rows[1].Name)
	}
	if rows[2].Sandbox != "sb2" || rows[2].Name != "b" {
		t.Fatalf("row[2]=%s/%s, want sb2/b", rows[2].Sandbox, rows[2].Name)
	}

	if rows[0].Proc.Sandbox != "sb1" || rows[0].Proc.Name != "c" {
		t.Fatalf("row[0].proc=%#v, want sandbox/name populated", rows[0].Proc)
	}
}

func TestCollectManagedProcRows_FilterSandbox(t *testing.T) {
	t.Parallel()

	st := state.Default()
	st.Procs["sb1"] = map[string]state.ManagedProc{
		"web": {Sandbox: "sb1", Name: "web"},
	}
	st.Procs["sb2"] = map[string]state.ManagedProc{
		"api": {Sandbox: "sb2", Name: "api"},
	}

	rows := collectManagedProcRows(st, "sb2")
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	if rows[0].Sandbox != "sb2" || rows[0].Name != "api" {
		t.Fatalf("row[0]=%s/%s, want sb2/api", rows[0].Sandbox, rows[0].Name)
	}
}
