package main

import (
	"errors"
	"os"

	"github.com/nayeemzen/agent-sandbox/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		var ec cli.ExitCodeError
		if errors.As(err, &ec) {
			os.Exit(ec.Code)
		}
		cli.HandleError(os.Stderr, err)
		os.Exit(1)
	}
}
