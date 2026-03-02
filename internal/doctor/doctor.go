package doctor

import (
	"fmt"
	"strings"
)

type Status string

const (
	Pass Status = "pass"
	Warn Status = "warn"
	Fail Status = "fail"
)

type CheckResult struct {
	ID          string `json:"id"`
	Status      Status `json:"status"`
	Summary     string `json:"summary"`
	Details     string `json:"details,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

func ExitCode(results []CheckResult) int {
	for _, r := range results {
		if r.Status == Fail {
			return 1
		}
	}
	return 0
}

func RenderHuman(results []CheckResult) string {
	var b strings.Builder

	for i, r := range results {
		if i > 0 {
			b.WriteByte('\n')
		}

		b.WriteString(fmt.Sprintf("%s %s: %s", strings.ToUpper(string(r.Status)), r.ID, r.Summary))

		if strings.TrimSpace(r.Details) != "" {
			b.WriteString("\n  ")
			b.WriteString(strings.ReplaceAll(strings.TrimRight(r.Details, "\n"), "\n", "\n  "))
		}

		if strings.TrimSpace(r.Remediation) != "" {
			b.WriteString("\n  Remediation: ")
			b.WriteString(strings.ReplaceAll(strings.TrimRight(r.Remediation, "\n"), "\n", "\n  "))
		}
	}

	return b.String()
}
