package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const currentVersion = 1

type ProcStatus string

const (
	ProcRunning ProcStatus = "running"
	ProcExited  ProcStatus = "exited"
	ProcUnknown ProcStatus = "unknown"
)

type ManagedProc struct {
	Sandbox   string     `json:"sandbox"`
	Name      string     `json:"name"`
	Command   []string   `json:"command"`
	PID       int        `json:"pid"`
	LogPath   string     `json:"log_path"`
	PidPath   string     `json:"pid_path"`
	StartedAt time.Time  `json:"started_at"`
	Status    ProcStatus `json:"status"`
}

type State struct {
	Version int `json:"version"`

	// Sandboxes are keyed by sandbox name.
	Sandboxes map[string]Sandbox `json:"sandboxes"`

	// Procs are keyed by sandbox name, then proc name.
	Procs map[string]map[string]ManagedProc `json:"procs"`
}

type Sandbox struct {
	Name      string    `json:"name"`
	Template  string    `json:"template"`
	CreatedAt time.Time `json:"created_at"`
	LastState string    `json:"last_state"`
}

func Default() State {
	return State{
		Version:   currentVersion,
		Sandboxes: map[string]Sandbox{},
		Procs:     map[string]map[string]ManagedProc{},
	}
}

func Load(path string) (State, error) {
	st := Default()

	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return State{}, err
	}

	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, err
	}

	if st.Version == 0 {
		st.Version = currentVersion
	}

	if st.Sandboxes == nil {
		st.Sandboxes = map[string]Sandbox{}
	}

	if st.Procs == nil {
		st.Procs = map[string]map[string]ManagedProc{}
	}

	return st, nil
}

func Save(path string, st State) error {
	if st.Version == 0 {
		st.Version = currentVersion
	}
	if st.Sandboxes == nil {
		st.Sandboxes = map[string]Sandbox{}
	}
	if st.Procs == nil {
		st.Procs = map[string]map[string]ManagedProc{}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
