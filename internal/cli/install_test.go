package cli

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDetectPackageManager(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		found map[string]bool
		want  packageManager
	}{
		{
			name:  "apt",
			found: map[string]bool{"apt-get": true},
			want:  packageManagerAPT,
		},
		{
			name:  "dnf",
			found: map[string]bool{"dnf": true},
			want:  packageManagerDNF,
		},
		{
			name:  "yum",
			found: map[string]bool{"yum": true},
			want:  packageManagerYUM,
		},
		{
			name:  "pacman",
			found: map[string]bool{"pacman": true},
			want:  packageManagerPacman,
		},
		{
			name:  "zypper",
			found: map[string]bool{"zypper": true},
			want:  packageManagerZypper,
		},
		{
			name:  "priority_apt_over_dnf",
			found: map[string]bool{"apt-get": true, "dnf": true},
			want:  packageManagerAPT,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pm, err := detectPackageManager(func(file string) (string, error) {
				if tc.found[file] {
					return "/usr/bin/" + file, nil
				}
				return "", errors.New("not found")
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pm != tc.want {
				t.Fatalf("pm=%q, want=%q", pm, tc.want)
			}
		})
	}
}

func TestDetectPackageManager_Unsupported(t *testing.T) {
	t.Parallel()

	_, err := detectPackageManager(func(string) (string, error) {
		return "", errors.New("not found")
	})
	if err == nil {
		t.Fatalf("expected unsupported package manager error")
	}
}

func TestInstallStepMarker(t *testing.T) {
	t.Parallel()

	if got := installStepMarker(installPending); got != "[ ]" {
		t.Fatalf("pending marker=%q", got)
	}
	if got := installStepMarker(installRunning); got != "[~]" {
		t.Fatalf("running marker=%q", got)
	}
	if got := installStepMarker(installComplete); got != "[x]" {
		t.Fatalf("complete marker=%q", got)
	}
	if got := installStepMarker(installSkipped); got != "[-]" {
		t.Fatalf("skipped marker=%q", got)
	}
	if got := installStepMarker(installFailed); got != "[!]" {
		t.Fatalf("failed marker=%q", got)
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	got := shellQuote("ab'cd")
	want := `'ab'"'"'cd'`
	if got != want {
		t.Fatalf("shellQuote=%q, want=%q", got, want)
	}
}

func TestPathContainsDir(t *testing.T) {
	t.Parallel()

	joined := filepath.Join("/usr/bin")
	if !pathContainsDir("/bin:"+joined+":/usr/local/bin", joined) {
		t.Fatalf("expected pathContainsDir to find %q", joined)
	}
	if pathContainsDir("/bin:/usr/local/bin", "/not-there") {
		t.Fatalf("did not expect pathContainsDir to find missing path")
	}
}

func TestShellProfilePathAndLine_Zsh(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("SHELL", "/bin/zsh")

	p, shell, line, err := shellProfilePathAndLine("/home/tester/.local/bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "/home/tester/.zshrc" {
		t.Fatalf("profile path=%q", p)
	}
	if shell != "zsh" {
		t.Fatalf("shell=%q", shell)
	}
	if line != `export PATH="$HOME/.local/bin:$PATH"` {
		t.Fatalf("line=%q", line)
	}
}

func TestShellProfilePathAndLine_Fish(t *testing.T) {
	t.Setenv("HOME", "/home/tester")
	t.Setenv("SHELL", "/usr/bin/fish")

	p, shell, line, err := shellProfilePathAndLine("/home/tester/.local/bin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != "/home/tester/.config/fish/config.fish" {
		t.Fatalf("profile path=%q", p)
	}
	if shell != "fish" {
		t.Fatalf("shell=%q", shell)
	}
	if line != `set -gx PATH /home/tester/.local/bin $PATH` {
		t.Fatalf("line=%q", line)
	}
}
