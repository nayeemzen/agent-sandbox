package cli

import (
	"strings"
	"testing"
)

func TestDefaultInteractiveShellCommand(t *testing.T) {
	t.Parallel()

	cmd := defaultInteractiveShellCommand()
	if len(cmd) != 3 {
		t.Fatalf("len(command)=%d, want 3", len(cmd))
	}
	if cmd[0] != "sh" || cmd[1] != "-lc" {
		t.Fatalf("unexpected launcher: %#v", cmd)
	}
	if !strings.Contains(cmd[2], "/bin/bash") {
		t.Fatalf("bootstrap script should prefer bash, got: %q", cmd[2])
	}
}

func TestInteractiveExecEnvironment(t *testing.T) {
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("COLORTERM", "truecolor")

	env := interactiveExecEnvironment()
	if env["TERM"] != "xterm-kitty" {
		t.Fatalf("TERM=%q, want xterm-kitty", env["TERM"])
	}
	if env["LANG"] != "en_US.UTF-8" {
		t.Fatalf("LANG=%q, want en_US.UTF-8", env["LANG"])
	}
	if env["COLORTERM"] != "truecolor" {
		t.Fatalf("COLORTERM=%q, want truecolor", env["COLORTERM"])
	}
}

func TestInteractiveExecEnvironment_DefaultTerm(t *testing.T) {
	t.Setenv("TERM", "")
	env := interactiveExecEnvironment()
	if env["TERM"] != "xterm-256color" {
		t.Fatalf("TERM=%q, want xterm-256color", env["TERM"])
	}
}
