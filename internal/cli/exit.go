package cli

import "fmt"

// ExitCodeError requests that the CLI process exit with the given code.
//
// Commands that return this should typically set SilenceErrors=true to avoid
// Cobra printing a generic "Error: ..." line for non-zero exit codes.
type ExitCodeError struct {
	Code int
}

func (e ExitCodeError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}
