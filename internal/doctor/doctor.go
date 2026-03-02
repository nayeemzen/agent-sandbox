package doctor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// Theme colors (match cli/style.go).
var (
	styledGreen  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	styledRed    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	styledYellow = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	styledGray   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styledCyan   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

// RenderStyledHuman renders check results with colored Unicode markers for TTY output.
func RenderStyledHuman(results []CheckResult) string {
	var b strings.Builder

	for i, r := range results {
		if i > 0 {
			b.WriteByte('\n')
		}

		var marker, label string
		switch r.Status {
		case Pass:
			marker = styledGreen.Render("✓")
			label = styledGreen.Render("PASS")
		case Warn:
			marker = styledYellow.Render("●")
			label = styledYellow.Render("WARN")
		case Fail:
			marker = styledRed.Render("✗")
			label = styledRed.Render("FAIL")
		default:
			marker = " "
			label = strings.ToUpper(string(r.Status))
		}

		b.WriteString(fmt.Sprintf("%s %s %s: %s", marker, label, r.ID, r.Summary))

		if strings.TrimSpace(r.Details) != "" {
			detail := strings.ReplaceAll(strings.TrimRight(r.Details, "\n"), "\n", "\n    ")
			b.WriteString("\n    ")
			b.WriteString(styledGray.Render(detail))
		}

		if strings.TrimSpace(r.Remediation) != "" {
			rem := strings.ReplaceAll(strings.TrimRight(r.Remediation, "\n"), "\n", "\n    ")
			b.WriteString("\n    ")
			b.WriteString(styledCyan.Render("Hint: "+rem))
		}
	}

	return b.String()
}
