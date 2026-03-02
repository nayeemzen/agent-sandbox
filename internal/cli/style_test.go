package cli

import (
	"bytes"
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
