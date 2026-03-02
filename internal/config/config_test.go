package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Version == 0 {
		t.Fatalf("expected Version to be set")
	}
	if cfg.Incus.UnixSocket == "" {
		t.Fatalf("expected Incus.UnixSocket default to be set")
	}
	if cfg.Incus.Project == "" {
		t.Fatalf("expected Incus.Project default to be set")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.json")

	in := Default()
	in.DefaultTemplate = "base"
	in.Incus.Project = "proj1"

	if err := Save(path, in); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	// Ensure the file was written.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if out.DefaultTemplate != "base" {
		t.Fatalf("DefaultTemplate = %q, want %q", out.DefaultTemplate, "base")
	}
	if out.Incus.Project != "proj1" {
		t.Fatalf("Incus.Project = %q, want %q", out.Incus.Project, "proj1")
	}
}
