package thread

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

// FuncRunner adapts a function to the otctl.Runner seam (for tests).
type FuncRunner func(ctx context.Context, args ...string) (string, error)

func (f FuncRunner) Run(ctx context.Context, args ...string) (string, error) {
	return f(ctx, args...)
}

// ExecRunner runs ot-ctl as a subprocess.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "ot-ctl", args...) //nolint:gosec // G204: ot-ctl subcommands are fixed API surface, not user shell input
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ot-ctl command failed: %w (output: %s)", err, string(output))
	}
	res := strings.TrimSpace(string(output))
	res = strings.TrimSuffix(res, "Done")
	return strings.TrimSpace(res), nil
}

// NewRunner returns the production or mock adapter for the given runtime mode.
func NewRunner(mode config.RuntimeMode) otctl.Runner {
	if mode.IsMock() {
		return NewMock()
	}
	return ExecRunner{}
}
