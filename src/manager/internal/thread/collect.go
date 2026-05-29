package thread

import (
	"context"
	"fmt"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

// Policy controls how ot-ctl collection failures are handled.
type Policy = otctl.Policy

const (
	PolicyBestEffort   = otctl.PolicyBestEffort
	PolicyStrict       = otctl.PolicyStrict
	PolicyBackupExport = otctl.PolicyBackupExport
)

type cmdAssignment struct {
	cmd *otctl.Command
	set func(string)
}

func (c *Client) runAssignments(ctx context.Context, policy Policy, assignments []cmdAssignment) ([]string, error) {
	otelAssignments := make([]otctl.Assignment, len(assignments))
	for i, a := range assignments {
		otelAssignments[i] = otctl.Assignment{Cmd: a.cmd, Set: a.set}
	}
	return otctl.CollectSequential(ctx, c.runner, otelAssignments, policy)
}

func (c *Client) runRequired(ctx context.Context, cmd *otctl.Command, label string) (string, error) {
	value, err := c.runner.Run(ctx, cmd.Args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return value, nil
}
