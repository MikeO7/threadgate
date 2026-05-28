// Package thread collects Thread network snapshots via ot-ctl.
package thread

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// Client orchestrates ot-ctl behind one interface: node info, datasets, diagnostics, topology snapshots, and backup export.
type Client struct {
	runner otctl.Runner
	policy Policy
}

// NewClient wires a Client with the given ot-ctl adapter and default collection policy.
func NewClient(runner otctl.Runner, policy Policy) *Client {
	if policy != PolicyStrict && policy != PolicyBackupExport {
		policy = PolicyBestEffort
	}
	return &Client{runner: runner, policy: policy}
}

// Policy returns the default collection policy for snapshot builds.
func (c *Client) Policy() Policy {
	return c.policy
}

// NodeInfo fetches node identity fields from ot-ctl using strict collection.
func (c *Client) NodeInfo(ctx context.Context) (NodeInfo, error) {
	var info NodeInfo
	_, err := c.runAssignments(ctx, PolicyStrict, []cmdAssignment{
		{&otctl.State, func(v string) { info.State = v }},
		{&otctl.Rloc16, func(v string) { info.Rloc16 = v }},
		{&otctl.ExtAddr, func(v string) { info.ExtAddress = v }},
		{&otctl.NetworkName, func(v string) { info.NetworkName = v }},
		{&otctl.PanID, func(v string) { info.PanID = v }},
	})
	if err != nil {
		return info, fmt.Errorf("node info: %w", err)
	}
	return info, nil
}

// Diagnostics fetches counters, addresses, and the raw neighbor table using strict collection.
func (c *Client) Diagnostics(ctx context.Context) (Diagnostics, error) {
	var diag Diagnostics
	_, err := c.runAssignments(ctx, PolicyStrict, []cmdAssignment{
		{&otctl.Counters, func(v string) { diag.Counters = splitLines(v) }},
		{&otctl.IPAddr, func(v string) { diag.IPAddresses = splitLines(v) }},
		{&otctl.NeighborTable, func(v string) { diag.NeighborTable = splitLines(v) }},
	})
	diag.Timestamp = time.Now().Format(time.RFC3339)
	if err != nil {
		return diag, fmt.Errorf("diagnostics: %w", err)
	}
	return diag, nil
}

func (c *Client) GetActiveDataset(ctx context.Context) (string, error) {
	return c.runRequired(ctx, &otctl.DatasetActive, "active dataset")
}

func (c *Client) SetActiveDataset(ctx context.Context, hexStr string) error {
	if _, err := c.runner.Run(ctx, append(otctl.DatasetSetActive.Args, hexStr)...); err != nil {
		return err
	}
	_, err := c.runner.Run(ctx, otctl.DatasetCommitActive.Args...)
	return err
}

func (c *Client) GetPendingDataset(ctx context.Context) (string, error) {
	return c.runRequired(ctx, &otctl.DatasetPending, "pending dataset")
}

func (c *Client) SetPendingDataset(ctx context.Context, hexStr string) error {
	if _, err := c.runner.Run(ctx, append(otctl.DatasetSetPending.Args, hexStr)...); err != nil {
		return err
	}
	_, err := c.runner.Run(ctx, otctl.DatasetCommitPending.Args...)
	return err
}

// BuildSnapshot collects ot-ctl output and builds the dashboard topology model.
func (c *Client) BuildSnapshot(ctx context.Context) (topology.Snapshot, error) {
	mode := topology.CollectBestEffort
	if c.policy == PolicyStrict {
		mode = topology.CollectStrict
	}
	return topology.Build(ctx, c.runner, topology.BuildOptions{Mode: mode})
}

// BackupMetadata holds Thread network credential fields for export.
type BackupMetadata struct {
	NetworkName string
	PanID       string
	Channel     string
	ExtAddress  string
}

// ExportBackupMetadata collects live network credentials from ot-ctl.
func (c *Client) ExportBackupMetadata(ctx context.Context) (BackupMetadata, error) {
	var meta BackupMetadata
	_, err := c.runAssignments(ctx, PolicyBackupExport, []cmdAssignment{
		{&otctl.NetworkName, func(v string) { meta.NetworkName = v }},
		{&otctl.PanID, func(v string) { meta.PanID = v }},
		{&otctl.Channel, func(v string) { meta.Channel = v }},
		{&otctl.ExtAddr, func(v string) { meta.ExtAddress = v }},
	})
	return meta, err
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// SplitLines splits ot-ctl multiline output into non-empty lines.
func SplitLines(s string) []string {
	return splitLines(s)
}
