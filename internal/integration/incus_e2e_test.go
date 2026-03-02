//go:build integration

package integration

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

func TestIncusEndToEnd_TemplateNewExecDetachLogsKill(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	bin := buildSandboxBinary(t)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.json")
	statePath := filepath.Join(tmp, "state.json")

	suffix := randHex(t, 4)
	templateName := "it-" + suffix
	sandboxName := "it-" + suffix + "-sb"

	t.Cleanup(func() {
		s, err := connectIncusForCleanup(ctx)
		if err != nil {
			return
		}
		_ = deleteInstance(ctx, s, sandboxName)
		_ = deleteTemplateImage(ctx, s, templateName)
	})

	// Template add.
	_, err := runCmd(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "template", "add", templateName, "images:alpine/3.20"})
	if err != nil {
		t.Fatalf("template add: %v", err)
	}

	// Sandbox new.
	_, err = runCmd(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "new", sandboxName, "--template", templateName})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	// Exec stdout and exit code 0.
	out, err := runCmd(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "exec", sandboxName, "--", "sh", "-lc", "echo hi"})
	if err != nil {
		t.Fatalf("exec echo: %v", err)
	}
	if !strings.Contains(out, "hi") {
		t.Fatalf("exec echo output missing %q; got: %q", "hi", out)
	}

	// Exec returns guest exit code.
	_, err = runCmdExpectExit(ctx, bin, 7, []string{"--config", cfgPath, "--state", statePath, "exec", sandboxName, "--", "sh", "-lc", "exit 7"})
	if err != nil {
		t.Fatalf("exec exit code: %v", err)
	}

	// Detach a managed proc and verify it appears in ps.
	_, err = runCmd(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "exec", sandboxName, "--detach", "--name", "demo", "--", "sh", "-lc", "echo start; sleep 1000"})
	if err != nil {
		t.Fatalf("exec --detach: %v", err)
	}

	psOut, err := runCmd(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "ps", sandboxName})
	if err != nil {
		t.Fatalf("ps: %v", err)
	}
	if !strings.Contains(psOut, "demo") {
		t.Fatalf("ps output missing %q; got: %q", "demo", psOut)
	}

	// Logs: ensure we can see "start" and then interrupt.
	if err := runLogsUntil(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "logs", sandboxName, "--proc", "demo"}, "start"); err != nil {
		t.Fatalf("logs: %v", err)
	}

	// Kill the managed proc.
	_, err = runCmd(ctx, bin, []string{"--config", cfgPath, "--state", statePath, "kill", sandboxName, "demo", "--force"})
	if err != nil {
		t.Fatalf("kill: %v", err)
	}
}

func buildSandboxBinary(t *testing.T) string {
	t.Helper()

	root := repoRoot(t)
	outDir := t.TempDir()
	bin := filepath.Join(outDir, "sandbox")

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/sandbox")
	cmd.Dir = root
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(b))
	}

	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to locate caller")
	}

	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("failed to find repo root (go.mod) starting from %s", file)
		}
		dir = parent
	}
}

func runCmd(ctx context.Context, bin string, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func runCmdExpectExit(ctx context.Context, bin string, wantExit int, args []string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()
	b, err := cmd.CombinedOutput()
	if err == nil {
		return string(b), fmt.Errorf("expected exit code %d, got 0", wantExit)
	}
	ee, ok := err.(*exec.ExitError)
	if !ok {
		return string(b), fmt.Errorf("expected ExitError, got %T: %v", err, err)
	}
	if ee.ExitCode() != wantExit {
		return string(b), fmt.Errorf("expected exit code %d, got %d; output=%q", wantExit, ee.ExitCode(), string(b))
	}
	return string(b), nil
}

func runLogsUntil(ctx context.Context, bin string, args []string, wantSubstring string) error {
	cmd := exec.CommandContext(ctx, bin, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Drain stderr so it doesn't block process exit.
	go func() { _, _ = io.Copy(io.Discard, stderr) }()

	lines := make(chan string, 16)
	readErr := make(chan error, 1)

	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			select {
			case lines <- sc.Text():
			default:
			}
		}
		select {
		case readErr <- sc.Err():
		default:
		}
	}()

	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return ctx.Err()
		case <-deadline.C:
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return fmt.Errorf("timed out waiting for logs to contain %q", wantSubstring)
		case line := <-lines:
			if strings.Contains(line, wantSubstring) {
				_ = cmd.Process.Signal(os.Interrupt)
				_ = cmd.Wait()
				return nil
			}
		case err := <-readErr:
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			if err == nil {
				return fmt.Errorf("logs exited before emitting %q", wantSubstring)
			}
			return err
		}
	}
}

func connectIncusForCleanup(ctx context.Context) (incusclient.InstanceServer, error) {
	return incusclient.ConnectIncusUnixWithContext(ctx, "/var/lib/incus/unix.socket", nil)
}

func deleteInstance(ctx context.Context, s incusclient.InstanceServer, name string) error {
	// Best-effort stop+delete.
	_, etag, err := s.GetInstanceState(name)
	if err == nil {
		if op, err := s.UpdateInstanceState(name, api.InstanceStatePut{Action: "stop", Timeout: 5, Force: true}, etag); err == nil {
			_ = op.WaitContext(ctx)
		}
	}

	if op, err := s.DeleteInstance(name); err == nil {
		_ = op.WaitContext(ctx)
	}

	return nil
}

func deleteTemplateImage(ctx context.Context, s incusclient.InstanceServer, templateName string) error {
	alias := "sandbox/" + templateName
	entry, _, err := s.GetImageAlias(alias)
	if err != nil {
		return nil
	}

	_ = s.DeleteImageAlias(alias)
	if op, err := s.DeleteImage(entry.Target); err == nil {
		_ = op.WaitContext(ctx)
	}
	return nil
}

func randHex(t *testing.T, nbytes int) string {
	t.Helper()
	b := make([]byte, nbytes)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
