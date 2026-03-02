package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const currentVersion = 1

type IncusConfig struct {
	UnixSocket string `json:"unix_socket"`
	RemoteURL  string `json:"remote_url"`
	Project    string `json:"project"`
	Insecure   bool   `json:"insecure"`
}

type Config struct {
	Version         int         `json:"version"`
	DefaultTemplate string      `json:"default_template"`
	Incus           IncusConfig `json:"incus"`
}

func Default() Config {
	return Config{
		Version: currentVersion,
		Incus: IncusConfig{
			UnixSocket: "/var/lib/incus/unix.socket",
			Project:    "default",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}

	if cfg.Version == 0 {
		cfg.Version = currentVersion
	}

	if cfg.Incus.UnixSocket == "" {
		cfg.Incus.UnixSocket = "/var/lib/incus/unix.socket"
	}
	if cfg.Incus.Project == "" {
		cfg.Incus.Project = "default"
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if cfg.Version == 0 {
		cfg.Version = currentVersion
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
