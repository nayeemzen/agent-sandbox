package doctor

import "testing"

func TestExitCode(t *testing.T) {
	if got := ExitCode([]CheckResult{{ID: "a", Status: Pass}}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}

	if got := ExitCode([]CheckResult{{ID: "a", Status: Warn}}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}

	if got := ExitCode([]CheckResult{{ID: "a", Status: Fail}}); got != 1 {
		t.Fatalf("expected exit 1, got %d", got)
	}
}

func TestRenderHuman(t *testing.T) {
	out := RenderHuman([]CheckResult{
		{
			ID:          "incus.api",
			Status:      Pass,
			Summary:     "reachable",
			Details:     "via unix socket",
			Remediation: "",
		},
		{
			ID:          "incus.metrics",
			Status:      Warn,
			Summary:     "disabled",
			Details:     "",
			Remediation: "enable metrics in Incus",
		},
	})

	if out == "" {
		t.Fatalf("expected non-empty output")
	}

	// Golden-ish sanity checks.
	want := []string{
		"PASS incus.api: reachable",
		"via unix socket",
		"WARN incus.metrics: disabled",
		"Remediation: enable metrics in Incus",
	}

	for _, s := range want {
		if !contains(out, s) {
			t.Fatalf("expected output to contain %q\n---\n%s\n---", s, out)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && (stringIndex(haystack, needle) >= 0))
}

// stringIndex avoids importing strings in this file to keep tests small and explicit.
func stringIndex(s, substr string) int {
	n := len(substr)
	if n == 0 {
		return 0
	}
	for i := 0; i+n <= len(s); i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
