package incus

import (
	"context"
	"fmt"
	"io"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type ExecOptions struct {
	Interactive bool
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

type ExecResult struct {
	ExitCode int
}

func Exec(ctx context.Context, s incusclient.InstanceServer, instance string, command []string, opts ExecOptions) (ExecResult, error) {
	dataDone := make(chan bool)

	execReq := api.InstanceExecPost{
		Command:      command,
		WaitForWS:    true,
		Interactive:  opts.Interactive,
		RecordOutput: false,
	}

	execArgs := &incusclient.InstanceExecArgs{
		Stdin:    opts.Stdin,
		Stdout:   opts.Stdout,
		Stderr:   opts.Stderr,
		DataDone: dataDone,
	}

	op, err := s.ExecInstance(instance, execReq, execArgs)
	if err != nil {
		return ExecResult{}, err
	}

	waitErr := op.WaitContext(ctx)
	if waitErr != nil {
		// If the caller cancelled, attempt to cancel the operation so the websocket
		// closes and we don't block waiting for DataDone.
		if ctx.Err() != nil {
			_ = op.Cancel()
		}

		select {
		case <-dataDone:
		case <-time.After(2 * time.Second):
		}

		return ExecResult{}, waitErr
	}

	// Ensure output is fully drained.
	select {
	case <-dataDone:
	case <-time.After(2 * time.Second):
	}

	codeAny, ok := op.Get().Metadata["return"]
	if !ok {
		// Some older servers may not provide it; treat as success.
		return ExecResult{ExitCode: 0}, nil
	}

	switch v := codeAny.(type) {
	case float64:
		return ExecResult{ExitCode: int(v)}, nil
	case int:
		return ExecResult{ExitCode: v}, nil
	case int64:
		return ExecResult{ExitCode: int(v)}, nil
	}

	// Best-effort fallback.
	return ExecResult{ExitCode: 0}, nil
}

func execInInstance(ctx context.Context, s incusclient.InstanceServer, instance string, command []string) error {
	res, err := Exec(ctx, s, instance, command, ExecOptions{})
	if err != nil {
		return err
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("exec in %s exited with %d", instance, res.ExitCode)
	}
	return nil
}
