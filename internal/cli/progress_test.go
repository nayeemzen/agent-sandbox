package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWithProgress_DisabledStillRuns(t *testing.T) {
	t.Parallel()

	called := false
	err := withProgress(&bytes.Buffer{}, false, "working", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("withProgress returned unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected callback to run")
	}
}

func TestWithProgress_EnabledRendersAndClears(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := withProgress(&buf, true, "creating template", func() error {
		time.Sleep(180 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Fatalf("withProgress returned unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "creating template") {
		t.Fatalf("expected progress message in output, got: %q", got)
	}
	if !strings.Contains(got, "\r") {
		t.Fatalf("expected carriage-return updates in output, got: %q", got)
	}
}
