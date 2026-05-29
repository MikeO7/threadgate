package otctl

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// Policy controls how ot-ctl collection failures are handled.
type Policy int

const (
	// PolicyBestEffort returns partial data with warnings when commands fail.
	PolicyBestEffort Policy = iota
	// PolicyStrict fails the entire collection on the first ot-ctl error.
	PolicyStrict
	// PolicyBackupExport tolerates optional metadata failures but reports errors.
	PolicyBackupExport
)

type cmdResult struct {
	label string
	value string
	err   error
}

// Assignment binds one ot-ctl command to a result setter (sequential collection).
type Assignment struct {
	Cmd *Command
	Set func(string)
}

// CollectParallel runs commands concurrently and returns label-keyed values.
func CollectParallel(ctx context.Context, runner Runner, commands []Command, policy Policy) (map[string]string, []string, error) {
	results := make([]cmdResult, len(commands))
	var wg sync.WaitGroup
	wg.Add(len(commands))
	for i, cmd := range commands {
		go func(i int, cmd Command) {
			defer wg.Done()
			out, err := runner.Run(ctx, cmd.Args...)
			results[i] = cmdResult{label: cmd.Label, value: out, err: err}
		}(i, cmd)
	}
	wg.Wait()
	return finalizeValues(results, policy)
}

// CollectSequential runs commands in order, applying setters on success.
func CollectSequential(ctx context.Context, runner Runner, assignments []Assignment, policy Policy) ([]string, error) {
	var warnings []string
	var firstErr error

	for _, item := range assignments {
		value, err := runner.Run(ctx, item.Cmd.Args...)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", item.Cmd.Label, err))
			if firstErr == nil {
				firstErr = err
			}
			if policy == PolicyStrict {
				return warnings, fmt.Errorf("%s: %w", item.Cmd.Label, err)
			}
			continue
		}
		item.Set(value)
	}

	return finalizeWarnings(policy, warnings, firstErr)
}

func finalizeValues(results []cmdResult, policy Policy) (map[string]string, []string, error) {
	var warnings []string
	values := make(map[string]string, len(results))
	var firstErr error
	for _, res := range results {
		if res.err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", res.label, res.err))
			if firstErr == nil {
				firstErr = res.err
			}
			if policy == PolicyStrict {
				return nil, warnings, fmt.Errorf("snapshot: %s: %w", res.label, res.err)
			}
			continue
		}
		values[res.label] = res.value
	}
	sort.Strings(warnings)
	if firstErr == nil {
		return values, warnings, nil
	}
	if policy == PolicyBestEffort {
		return values, warnings, fmt.Errorf("collection partial: %w", firstErr)
	}
	if policy == PolicyBackupExport {
		return values, warnings, fmt.Errorf("backup metadata: %w", firstErr)
	}
	return values, warnings, nil
}

func finalizeWarnings(policy Policy, warnings []string, firstErr error) ([]string, error) {
	sort.Strings(warnings)
	if firstErr == nil {
		return warnings, nil
	}
	switch policy {
	case PolicyBestEffort:
		return warnings, fmt.Errorf("collection partial: %w", firstErr)
	case PolicyBackupExport:
		return warnings, fmt.Errorf("backup metadata: %w", firstErr)
	default:
		return warnings, nil
	}
}
