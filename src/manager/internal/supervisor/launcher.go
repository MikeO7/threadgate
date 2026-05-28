package supervisor

import (
	"context"
	"os/exec"
)

// ProcessLauncher starts subprocesses (exec adapter in prod, fakes in tests).
type ProcessLauncher interface {
	CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// ExecLauncher runs real subprocesses via os/exec.
type ExecLauncher struct{}

func (ExecLauncher) CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...)
}
