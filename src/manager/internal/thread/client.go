// Package thread collects Thread network snapshots via ot-ctl.
package thread

import (
	"context"
	"fmt"
	"strconv"
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

// ScanChannels executes an energy scan via ot-ctl and parses/analyzes results.
func (c *Client) ScanChannels(ctx context.Context) ([]ChannelScanResult, error) {
	output, err := c.runner.Run(ctx, otctl.ScanEnergy.Args...)
	if err != nil {
		return nil, fmt.Errorf("scan energy command failed: %w", err)
	}

	return ParseScanEnergyOutput(output)
}

// ParseScanEnergyOutput parses raw scan energy output into structured ChannelScanResult.
func ParseScanEnergyOutput(output string) ([]ChannelScanResult, error) {
	lines := strings.Split(output, "\n")
	var results []ChannelScanResult

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		// Skip header line or separator line
		if strings.Contains(line, "Ch") || strings.Contains(line, "RSSI") || strings.Contains(line, "+") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		chanStr := strings.TrimSpace(parts[1])
		rssiStr := strings.TrimSpace(parts[2])

		channel, err := strconv.Atoi(chanStr)
		if err != nil {
			continue
		}

		rssi, err := strconv.Atoi(rssiStr)
		if err != nil {
			continue
		}

		rating, recommendation := AnalyzeChannel(channel, rssi)

		results = append(results, ChannelScanResult{
			Channel:        channel,
			RSSI:           rssi,
			Rating:         rating,
			Recommendation: recommendation,
		})
	}

	return results, nil
}

// AnalyzeChannel evaluates the channel RSSI and provides smart rating and Wi-Fi co-existence guidance.
func AnalyzeChannel(channel int, rssi int) (string, string) {
	var rating string
	var reason string

	if rssi <= -85 {
		rating = "Excellent"
	} else if rssi <= -75 {
		rating = "Good"
	} else if rssi <= -65 {
		rating = "Fair"
	} else {
		rating = "Poor"
	}

	switch channel {
	case 11, 12, 13, 14:
		reason = "Overlaps with 2.4GHz Wi-Fi Channel 1. Strong router traffic will introduce noise."
	case 15:
		reason = "Quiet gap between 2.4GHz Wi-Fi Channels 1 and 6. Highly recommended!"
	case 16, 17, 18, 19:
		reason = "Overlaps with 2.4GHz Wi-Fi Channel 6 (common router default frequency)."
	case 20:
		reason = "Quiet gap between 2.4GHz Wi-Fi Channels 6 and 11. Excellent alternative channel."
	case 21, 22, 23, 24:
		reason = "Overlaps with 2.4GHz Wi-Fi Channel 11. Heavy traffic is common here."
	case 25:
		reason = "Sits above 2.4GHz Wi-Fi Channel 11. Highly clear and reliable frequency."
	case 26:
		reason = "Completely clear of standard Wi-Fi channels. Note that legacy hardware may have reduced power here."
	default:
		reason = "Standard IEEE 802.15.4 channel frequency."
	}

	var advice string
	switch rating {
	case "Excellent":
		advice = fmt.Sprintf("Excellent (noise: %d dBm). %s", rssi, reason)
	case "Good":
		advice = fmt.Sprintf("Good (noise: %d dBm). %s", rssi, reason)
	case "Fair":
		advice = fmt.Sprintf("Fair (noise: %d dBm). %s", rssi, reason)
	case "Poor":
		advice = fmt.Sprintf("Poor (noise: %d dBm). %s Avoid this congested channel.", rssi, reason)
	}

	return rating, advice
}
