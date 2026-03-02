package cli

import "testing"

func TestComputeSetupPlan_LocalVsRemote(t *testing.T) {
	t.Parallel()

	local := computeSetupPlan(&GlobalOptions{IncusRemoteURL: ""})
	if !local.LocalMode {
		t.Fatalf("expected LocalMode=true for empty remote URL")
	}

	remote := computeSetupPlan(&GlobalOptions{IncusRemoteURL: "https://example:8443"})
	if remote.LocalMode {
		t.Fatalf("expected LocalMode=false for non-empty remote URL")
	}
}
