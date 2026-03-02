package cli

import (
	"testing"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

func TestMonitorStateRank(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status string
		want   int
	}{
		{status: "Running", want: 0},
		{status: "frozen", want: 1},
		{status: "Stopped", want: 2},
		{status: "Error", want: 3},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			t.Parallel()
			if got := monitorStateRank(tc.status); got != tc.want {
				t.Fatalf("monitorStateRank(%q)=%d, want %d", tc.status, got, tc.want)
			}
		})
	}
}

func TestSortSandboxesForMonitor(t *testing.T) {
	t.Parallel()

	sandboxes := []incus.Sandbox{
		{Name: "zeta", Status: "Stopped"},
		{Name: "beta", Status: "Running"},
		{Name: "alpha", Status: "Running"},
		{Name: "omega", Status: "Frozen"},
		{Name: "delta", Status: "Error"},
	}

	sortSandboxesForMonitor(sandboxes)

	want := []string{"alpha", "beta", "omega", "zeta", "delta"}
	if len(sandboxes) != len(want) {
		t.Fatalf("len=%d, want %d", len(sandboxes), len(want))
	}
	for i, got := range sandboxes {
		if got.Name != want[i] {
			t.Fatalf("idx %d: got %q, want %q", i, got.Name, want[i])
		}
	}
}

func TestFilterSandboxesForMonitor_DefaultHidesStopped(t *testing.T) {
	t.Parallel()

	sandboxes := []incus.Sandbox{
		{Name: "run1", Status: "Running"},
		{Name: "frz1", Status: "Frozen"},
		{Name: "stp1", Status: "Stopped"},
		{Name: "err1", Status: "Error"},
	}

	got := filterSandboxesForMonitor(sandboxes, false)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].Name != "run1" || got[1].Name != "frz1" {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestFilterSandboxesForMonitor_AllShowsEverything(t *testing.T) {
	t.Parallel()

	sandboxes := []incus.Sandbox{
		{Name: "run1", Status: "Running"},
		{Name: "frz1", Status: "Frozen"},
		{Name: "stp1", Status: "Stopped"},
	}

	got := filterSandboxesForMonitor(sandboxes, true)
	if len(got) != len(sandboxes) {
		t.Fatalf("len=%d, want %d", len(got), len(sandboxes))
	}
}
