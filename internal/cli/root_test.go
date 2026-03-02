package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpIncludesPlannedCommands(t *testing.T) {
	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to succeed, got error: %v", err)
	}

	out := buf.String()
	wantSubstrings := []string{
		"setup",
		"doctor",
		"init",
		"template",
		"new",
		"ls",
		"exec",
		"logs",
		"ps",
		"kill",
		"pause",
		"resume",
		"stop",
		"start",
		"delete",
		"monitor",
		"--json",
	}

	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Fatalf("help output missing %q\n---\n%s\n---", s, out)
		}
	}
}

func TestTemplateHelpIncludesSubcommands(t *testing.T) {
	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"template", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected help to succeed, got error: %v", err)
	}

	out := buf.String()
	wantSubstrings := []string{
		"add",
		"ls",
		"rm",
		"default",
	}

	for _, s := range wantSubstrings {
		if !strings.Contains(out, s) {
			t.Fatalf("template help output missing %q\n---\n%s\n---", s, out)
		}
	}
}
