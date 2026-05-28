package thread

import (
	"context"
	"fmt"
	"sort"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

// Policy controls how ot-ctl collection failures are handled.
type Policy int

const (
	// PolicyBestEffort returns partial data with warnings when commands fail.
	PolicyBestEffort Policy = iota
	// PolicyStrict fails the entire collection on the first ot-ctl error.
	PolicyStrict
	// PolicyBackupExport tolerates optional metadata failures but requires active dataset.
	PolicyBackupExport
)

type cmdAssignment struct {
	cmd *otctl.Command
	set func(string)
}

type cmdOutcome struct {
	label string
	value string
	err   error
}

func (c *Client) runAssignments(ctx context.Context, policy Policy, assignments []cmdAssignment) ([]string, error) {
	var warnings []string
	var firstErr error

	for _, item := range assignments {
		value, err := c.runner.Run(ctx, item.cmd.Args...)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", item.cmd.Label, err))
			if firstErr == nil {
				firstErr = err
			}
			if policy == PolicyStrict {
				return warnings, fmt.Errorf("%s: %w", item.cmd.Label, err)
			}
			continue
		}
		item.set(value)
	}

	sort.Strings(warnings)
	if firstErr != nil && policy == PolicyBestEffort {
		return warnings, fmt.Errorf("collection partial: %w", firstErr)
	}
	if firstErr != nil && policy == PolicyBackupExport {
		return warnings, fmt.Errorf("backup metadata: %w", firstErr)
	}
	return warnings, nil
}

func (c *Client) runRequired(ctx context.Context, cmd *otctl.Command, label string) (string, error) {
	value, err := c.runner.Run(ctx, cmd.Args...)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	return value, nil
}
