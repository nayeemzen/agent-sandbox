package incus

import "strings"

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Incus client doesn't expose typed errors consistently; best-effort.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found")
}

func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists")
}

type SandboxExistsError struct {
	Name    string
	Managed bool
	Status  string
}

func (e *SandboxExistsError) Error() string {
	name := strings.TrimSpace(e.Name)
	if name == "" {
		name = "sandbox"
	}
	if e.Managed {
		return `sandbox "` + name + `" already exists`
	}
	return `instance "` + name + `" already exists`
}
