// Package topology builds mesh graphs and traffic paths from ot-ctl tables.
package topology

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

type cmdResult struct {
	label string
	value string
	err   error
}

func runSnapshotCommands(ctx context.Context, runner otctl.Runner) []cmdResult {
	commands := otctl.SnapshotCommands
	results := make([]cmdResult, len(commands))
	var wg sync.WaitGroup
	wg.Add(len(commands))
	for i, cmd := range commands {
		go func(i int, cmd otctl.Command) {
			defer wg.Done()
			out, err := runner.Run(ctx, cmd.Args...)
			results[i] = cmdResult{label: cmd.Label, value: out, err: err}
		}(i, cmd)
	}
	wg.Wait()
	return results
}

func collectSnapshotValues(results []cmdResult, mode CollectMode) (map[string]string, []string, error) {
	var warnings []string
	values := make(map[string]string, len(results))
	for _, res := range results {
		if res.err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", res.label, res.err))
			if mode == CollectStrict {
				return nil, warnings, fmt.Errorf("snapshot: %s: %w", res.label, res.err)
			}
			continue
		}
		values[res.label] = res.value
	}
	sort.Strings(warnings)
	return values, warnings, nil
}

func normalizeGatewayRloc(rloc string) string {
	if rloc == "" {
		return ""
	}
	if !strings.HasPrefix(rloc, "0x") {
		return "0x" + rloc
	}
	return rloc
}

func assembleSnapshot(values map[string]string, warnings []string) Snapshot {
	var snap Snapshot
	snap.State = values[otctl.LabelState]
	snap.NetworkName = values[otctl.LabelNetworkName]
	snap.ExtAddress = values[otctl.LabelExtAddr]
	snap.PanID = values[otctl.LabelPanID]
	snap.Channel = values[otctl.LabelChannel]
	snap.Rloc16 = values[otctl.LabelRloc16]
	snap.ActiveDataset = values[otctl.LabelDataset]

	for _, ip := range splitLines(values[otctl.LabelIPAddr]) {
		if ip != "" {
			snap.IPAddresses = append(snap.IPAddresses, ip)
		}
	}

	neighbors := parseNeighborTable(values[otctl.LabelNeighborTable])
	children := parseChildTable(values[otctl.LabelChildTable])
	routers := parseRouterTable(values[otctl.LabelRouterTable])
	snap.Counters = parseCounters(values[otctl.LabelCounters])
	snap.LeaderData = parseLeaderData(values[otctl.LabelLeaderData])
	snap.Prefixes = parsePrefixTable(values[otctl.LabelPrefixTable])

	gatewayRloc := normalizeGatewayRloc(snap.Rloc16)

	snap.Neighbors = mergeMeshNodes(neighbors, children, routers)
	snap.MeshLinks = buildMeshLinks(gatewayRloc, neighbors, children, routers)
	snap.RoutingTree = buildRoutingTree(gatewayRloc, snap.MeshLinks)
	snap.TrafficPaths = buildAllTrafficPaths(gatewayRloc, snap.Neighbors, snap.MeshLinks)
	snap.Warnings = warnings
	return snap
}

// Build collects ot-ctl output and builds the topology snapshot.
func Build(ctx context.Context, runner otctl.Runner, opts BuildOptions) (Snapshot, error) {
	results := runSnapshotCommands(ctx, runner)
	values, warnings, err := collectSnapshotValues(results, opts.Mode)
	if err != nil {
		return Snapshot{Warnings: warnings}, err
	}

	snap := assembleSnapshot(values, warnings)
	if len(warnings) > 0 && opts.Mode == CollectBestEffort {
		return snap, fmt.Errorf("snapshot partial: %s", warnings[0])
	}
	return snap, nil
}

// BuildFromTables builds a snapshot from raw ot-ctl table output (for tests).
func BuildFromTables(gatewayRloc, rawNeighbors, rawChildren, rawRouters string) Snapshot {
	neighbors := parseNeighborTable(rawNeighbors)
	children := parseChildTable(rawChildren)
	routers := parseRouterTable(rawRouters)
	gatewayRloc = normalizeGatewayRloc(gatewayRloc)
	meshNodes := mergeMeshNodes(neighbors, children, routers)
	links := buildMeshLinks(gatewayRloc, neighbors, children, routers)
	return Snapshot{
		Rloc16:       gatewayRloc,
		Neighbors:    meshNodes,
		MeshLinks:    links,
		RoutingTree:  buildRoutingTree(gatewayRloc, links),
		TrafficPaths: buildAllTrafficPaths(gatewayRloc, meshNodes, links),
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
