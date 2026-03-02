package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestColorizeStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string // substring that must appear
	}{
		{"Running", "Running"},
		{"Stopped", "Stopped"},
		{"Frozen", "Frozen"},
		{"Unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := colorizeStatus(tt.input)
			if !strings.Contains(got, tt.want) {
				t.Fatalf("colorizeStatus(%q) = %q, missing %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestColorizeProcStatus(t *testing.T) {
	t.Parallel()

	got := colorizeProcStatus("running")
	if !strings.Contains(got, "running") {
		t.Fatalf("colorizeProcStatus(running) = %q, missing 'running'", got)
	}

	got = colorizeProcStatus("exited")
	if !strings.Contains(got, "exited") {
		t.Fatalf("colorizeProcStatus(exited) = %q, missing 'exited'", got)
	}
}

func TestPlainTable(t *testing.T) {
	t.Parallel()

	headers := []string{"A", "B", "C"}
	rows := [][]string{{"1", "2", "3"}, {"x", "y", "z"}}

	got := plainTable(headers, rows)
	want := "A\tB\tC\n1\t2\t3\nx\ty\tz\n"
	if got != want {
		t.Fatalf("plainTable mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRenderTable_NonTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{{"foo", "Running"}}

	renderTable(&buf, headers, rows)

	got := buf.String()
	// Non-TTY: should be plain tab-separated, no box-drawing characters.
	if strings.ContainsAny(got, "─│┌┐└┘") {
		t.Fatalf("renderTable to non-TTY unexpectedly contains borders: %q", got)
	}
	if !strings.Contains(got, "NAME\tSTATUS") {
		t.Fatalf("renderTable to non-TTY missing header line: %q", got)
	}
}

func TestRenderError_NonTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderError(&buf, "something broke", "try again")
	got := buf.String()
	if !strings.Contains(got, "Error: something broke") {
		t.Fatalf("renderError missing message: %q", got)
	}
	if !strings.Contains(got, "Hint: try again") {
		t.Fatalf("renderError missing hint: %q", got)
	}
}

func TestRenderError_NoHint(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderError(&buf, "something broke", "")
	got := buf.String()
	if !strings.Contains(got, "Error: something broke") {
		t.Fatalf("renderError missing message: %q", got)
	}
	if strings.Contains(got, "Hint:") {
		t.Fatalf("renderError should not have hint: %q", got)
	}
}

func TestRenderSuccess_NonTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderSuccess(&buf, "install complete")
	got := buf.String()
	if !strings.Contains(got, "install complete") {
		t.Fatalf("renderSuccess missing message: %q", got)
	}
}

func TestRenderWarning_NonTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderWarning(&buf, "disk almost full")
	got := buf.String()
	if !strings.Contains(got, "Warning: disk almost full") {
		t.Fatalf("renderWarning missing message: %q", got)
	}
}

func TestRenderHint_NonTTY(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	renderHint(&buf, "run sandbox doctor")
	got := buf.String()
	if !strings.Contains(got, "Hint: run sandbox doctor") {
		t.Fatalf("renderHint missing message: %q", got)
	}
}

func TestHandleError_CLIError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := newCLIError("bad input", "check the docs")
	HandleError(&buf, err)
	got := buf.String()
	if !strings.Contains(got, "Error: bad input") {
		t.Fatalf("HandleError missing message: %q", got)
	}
	if !strings.Contains(got, "Hint: check the docs") {
		t.Fatalf("HandleError missing hint: %q", got)
	}
}

func TestHandleError_PlainError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	HandleError(&buf, fmt.Errorf("generic error"))
	got := buf.String()
	if !strings.Contains(got, "Error: generic error") {
		t.Fatalf("HandleError missing message: %q", got)
	}
	if strings.Contains(got, "Hint:") {
		t.Fatalf("HandleError should not have hint: %q", got)
	}
}

func TestInstallStepStyledMarker_TTY(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status installStepStatus
		want   string
	}{
		{installComplete, "✓"},
		{installFailed, "✗"},
		{installRunning, "●"},
		{installSkipped, "−"},
		{installPending, "◌"},
	}
	for _, tc := range cases {
		got := installStepStyledMarker(tc.status, true)
		if !strings.Contains(got, tc.want) {
			t.Errorf("installStepStyledMarker(%s, true) = %q, missing %q", tc.status, got, tc.want)
		}
	}
}

func TestInstallStepStyledMarker_NonTTY(t *testing.T) {
	t.Parallel()

	// Non-TTY should fall back to ASCII markers.
	if got := installStepStyledMarker(installComplete, false); got != "[x]" {
		t.Fatalf("non-TTY complete marker = %q, want [x]", got)
	}
	if got := installStepStyledMarker(installPending, false); got != "[ ]" {
		t.Fatalf("non-TTY pending marker = %q, want [ ]", got)
	}
}
