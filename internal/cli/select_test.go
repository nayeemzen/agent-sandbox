package cli

import (
	"testing"
	"time"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func TestSandboxOptionsFromState_UnionAndSorted(t *testing.T) {
	t.Parallel()

	st := state.Default()
	st.Sandboxes["b"] = state.Sandbox{Name: "b"}
	st.Sandboxes["a"] = state.Sandbox{Name: "a"}
	st.Procs["c"] = map[string]state.ManagedProc{
		"web": {Sandbox: "c", Name: "web", StartedAt: time.Now().UTC()},
	}

	got := sandboxOptionsFromState(st)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Value != "a" || got[1].Value != "b" || got[2].Value != "c" {
		t.Fatalf("unexpected order/values: %#v", got)
	}
}

func TestProcOptionsFromState_Sorted(t *testing.T) {
	t.Parallel()

	st := state.Default()
	st.Procs["sb1"] = map[string]state.ManagedProc{
		"z": {Sandbox: "sb1", Name: "z"},
		"a": {Sandbox: "sb1", Name: "a"},
	}

	got := procOptionsFromState(st, "sb1")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Value != "a" || got[1].Value != "z" {
		t.Fatalf("unexpected order/values: %#v", got)
	}
}

func TestSandboxOptionsFromIncus_SortedByName(t *testing.T) {
	t.Parallel()

	sandboxes := []incus.Sandbox{
		{Name: "zeta", Status: "Running"},
		{Name: "alpha", Status: "Frozen"},
	}

	got := sandboxOptionsFromIncus(sandboxes)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Value != "alpha" || got[1].Value != "zeta" {
		t.Fatalf("unexpected order/values: %#v", got)
	}
}
