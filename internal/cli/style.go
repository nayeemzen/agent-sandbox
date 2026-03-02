package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"
)

// Theme colors.
var (
	colorGreen  = lipgloss.Color("2")  // Running
	colorRed    = lipgloss.Color("1")  // Stopped / exited
	colorYellow = lipgloss.Color("3")  // Frozen (paused)
	colorCyan   = lipgloss.Color("6")  // info values
	colorGray   = lipgloss.Color("8")  // muted / borders
	colorWhite  = lipgloss.Color("15") // headers
)

// Reusable styles.
var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
	boldGreen    = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	labelStyle   = lipgloss.NewStyle().Bold(true)
	cyanStyle    = lipgloss.NewStyle().Foreground(colorCyan)
	dimStyle     = lipgloss.NewStyle().Foreground(colorGray)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorRed)
	warnStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	hintStyle    = lipgloss.NewStyle().Foreground(colorCyan)
)

// colorizeStatus colors a sandbox state string for TTY output.
func colorizeStatus(status string) string {
	switch strings.ToLower(status) {
	case "running":
		return lipgloss.NewStyle().Foreground(colorGreen).Render(status)
	case "stopped":
		return lipgloss.NewStyle().Foreground(colorRed).Render(status)
	case "frozen":
		return lipgloss.NewStyle().Foreground(colorYellow).Render(status)
	default:
		return status
	}
}

// colorizeProcStatus colors a managed-proc status string for TTY output.
func colorizeProcStatus(status string) string {
	switch strings.ToLower(status) {
	case "running":
		return lipgloss.NewStyle().Foreground(colorGreen).Render(status)
	case "exited":
		return lipgloss.NewStyle().Foreground(colorRed).Render(status)
	default:
		return lipgloss.NewStyle().Foreground(colorGray).Render(status)
	}
}

// termWidth returns the current terminal width, or 80 as a fallback.
func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// styledTable renders a bordered, styled table string that fills the terminal width.
func styledTable(headers []string, rows [][]string) string {
	t := table.New().
		Width(termWidth()).
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(colorGray)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return t.String()
}

// plainTable renders a plain tab-separated table (for piped output).
func plainTable(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString(strings.Join(headers, "\t"))
	b.WriteByte('\n')
	for _, row := range rows {
		b.WriteString(strings.Join(row, "\t"))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderTable writes a styled or plain table based on whether w is a TTY.
func renderTable(w io.Writer, headers []string, rows [][]string) {
	if isTTY(w) {
		fmt.Fprint(w, styledTable(headers, rows))
	} else {
		fmt.Fprint(w, plainTable(headers, rows))
	}
}

// --- Styled message helpers (TTY-aware) ---

// renderError writes a styled error message. If hint is non-empty a cyan Hint line follows.
func renderError(w io.Writer, msg string, hint string) {
	if isTTY(w) {
		fmt.Fprintln(w, errorStyle.Render("Error:")+" "+msg)
	} else {
		fmt.Fprintln(w, "Error: "+msg)
	}
	if strings.TrimSpace(hint) != "" {
		renderHint(w, hint)
	}
}

// renderSuccess writes a green "✓ msg" on TTY, or plain "msg" otherwise.
func renderSuccess(w io.Writer, msg string) {
	if isTTY(w) {
		fmt.Fprintln(w, successStyle.Render("✓")+" "+msg)
	} else {
		fmt.Fprintln(w, msg)
	}
}

// renderWarning writes a styled warning message.
func renderWarning(w io.Writer, msg string) {
	if isTTY(w) {
		fmt.Fprintln(w, warnStyle.Render("Warning:")+" "+msg)
	} else {
		fmt.Fprintln(w, "Warning: "+msg)
	}
}

// renderHint writes a standalone cyan hint line.
func renderHint(w io.Writer, msg string) {
	if isTTY(w) {
		fmt.Fprintln(w, hintStyle.Render("Hint:")+" "+msg)
	} else {
		fmt.Fprintln(w, "Hint: "+msg)
	}
}

// CLIError is an error with a user-facing hint for recovery.
type CLIError struct {
	Message string
	Hint    string
}

func (e *CLIError) Error() string { return e.Message }

func newCLIError(msg, hint string) *CLIError {
	return &CLIError{Message: msg, Hint: hint}
}

// HandleError renders any error to w. It extracts the hint from CLIError if present.
func HandleError(w io.Writer, err error) {
	var ce *CLIError
	if errors.As(err, &ce) {
		renderError(w, ce.Message, ce.Hint)
		return
	}
	renderError(w, err.Error(), "")
}

// installStepStyledMarker returns a colored Unicode marker on TTY, ASCII on non-TTY.
func installStepStyledMarker(status installStepStatus, tty bool) string {
	if !tty {
		return installStepMarker(status)
	}
	switch status {
	case installComplete:
		return lipgloss.NewStyle().Bold(true).Foreground(colorGreen).Render("✓")
	case installFailed:
		return lipgloss.NewStyle().Bold(true).Foreground(colorRed).Render("✗")
	case installRunning:
		return lipgloss.NewStyle().Bold(true).Foreground(colorYellow).Render("●")
	case installSkipped:
		return dimStyle.Render("−")
	default: // pending
		return dimStyle.Render("◌")
	}
}
