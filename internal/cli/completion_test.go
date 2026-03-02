package cli

import (
	"os"
	"testing"
)

func TestCompletionShell_Explicit(t *testing.T) {
	t.Parallel()

	got, err := completionShell([]string{"zsh"})
	if err != nil {
		t.Fatalf("completionShell returned error: %v", err)
	}
	if got != "zsh" {
		t.Fatalf("got %q, want %q", got, "zsh")
	}
}

func TestCompletionShell_AutoFromEnv(t *testing.T) {
	t.Parallel()

	old := os.Getenv("SHELL")
	t.Cleanup(func() { _ = os.Setenv("SHELL", old) })
	_ = os.Setenv("SHELL", "/usr/bin/zsh")

	got, err := completionShell(nil)
	if err != nil {
		t.Fatalf("completionShell returned error: %v", err)
	}
	if got != "zsh" {
		t.Fatalf("got %q, want %q", got, "zsh")
	}
}

func TestCompletionShell_AutoPwshAlias(t *testing.T) {
	t.Parallel()

	old := os.Getenv("SHELL")
	t.Cleanup(func() { _ = os.Setenv("SHELL", old) })
	_ = os.Setenv("SHELL", "/usr/bin/pwsh")

	got, err := completionShell(nil)
	if err != nil {
		t.Fatalf("completionShell returned error: %v", err)
	}
	if got != "powershell" {
		t.Fatalf("got %q, want %q", got, "powershell")
	}
}

func TestCompletionShell_EmptyEnv(t *testing.T) {
	t.Parallel()

	old := os.Getenv("SHELL")
	t.Cleanup(func() { _ = os.Setenv("SHELL", old) })
	_ = os.Setenv("SHELL", "")

	_, err := completionShell(nil)
	if err == nil {
		t.Fatalf("expected error when SHELL is empty")
	}
}
