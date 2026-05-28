package api

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

// OtCtl is the seam for local ot-ctl execution (exec adapter in prod, mock adapter in tests).
type OtCtl = otctl.Runner

// ExecOtCtl runs ot-ctl as a subprocess.
type ExecOtCtl struct{}

func (ExecOtCtl) Run(ctx context.Context, args ...string) (string, error) {
	return runOtCtlWithContext(ctx, args...)
}

// NewOtCtl returns the production or mock adapter for the given mode.
func NewOtCtl(mockMode bool) OtCtl {
	if mockMode {
		return NewMockOtCtl()
	}
	return ExecOtCtl{}
}

// FuncOtCtl adapts a function to the OtCtl interface (for tests).
type FuncOtCtl func(ctx context.Context, args ...string) (string, error)

func (f FuncOtCtl) Run(ctx context.Context, args ...string) (string, error) {
	return f(ctx, args...)
}

func runOtCtlWithContext(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "ot-ctl", args...) //nolint:gosec // G204: ot-ctl subcommands are fixed API surface, not user shell input
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ot-ctl command failed: %w (output: %s)", err, string(output))
	}
	res := strings.TrimSpace(string(output))
	res = strings.TrimSuffix(res, "Done")
	return strings.TrimSpace(res), nil
}
