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
