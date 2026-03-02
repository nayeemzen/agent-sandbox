package cli

import (
	"context"

	incusclient "github.com/lxc/incus/v6/client"

	"github.com/nayeemzen/agent-sandbox/internal/config"
	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/paths"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func configPath(opts *GlobalOptions) (string, error) {
	if opts.ConfigPath != "" {
		return opts.ConfigPath, nil
	}
	return paths.ConfigFile()
}

func statePath(opts *GlobalOptions) (string, error) {
	if opts.StatePath != "" {
		return opts.StatePath, nil
	}
	return paths.StateFile()
}

func loadConfig(opts *GlobalOptions) (config.Config, string, error) {
	path, err := configPath(opts)
	if err != nil {
		return config.Config{}, "", err
	}

	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, "", err
	}

	return cfg, path, nil
}

func saveConfig(path string, cfg config.Config) error {
	return config.Save(path, cfg)
}

func loadState(opts *GlobalOptions) (state.State, string, error) {
	path, err := statePath(opts)
	if err != nil {
		return state.State{}, "", err
	}

	st, err := state.Load(path)
	if err != nil {
		return state.State{}, "", err
	}

	return st, path, nil
}

func saveState(path string, st state.State) error {
	return state.Save(path, st)
}

func connectIncus(ctx context.Context, opts *GlobalOptions) (incusclient.InstanceServer, error) {
	s, err := incus.Connect(ctx, incus.ConnectOptions{
		UnixSocket:         opts.IncusUnixSocket,
		RemoteURL:          opts.IncusRemoteURL,
		InsecureSkipVerify: opts.IncusInsecure,
	})
	if err != nil {
		return nil, err
	}

	if opts.IncusProject != "" && opts.IncusProject != "default" {
		s = s.UseProject(opts.IncusProject)
	}

	return s, nil
}
