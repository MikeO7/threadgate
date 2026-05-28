package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type snapshotCommand struct {
	label string
	args  []string
}

type cmdResult struct {
	label string
	value string
	err   error
}

var snapshotCommands = []snapshotCommand{
	{otctlCmdState, []string{otctlCmdState}},
	{otctlCmdNetworkName, []string{otctlCmdNetworkName}},
	{otctlCmdExtAddr, []string{otctlCmdExtAddr}},
	{otctlCmdPanID, []string{otctlCmdPanID}},
	{otctlCmdChannel, []string{otctlCmdChannel}},
	{otctlCmdRloc16, []string{otctlCmdRloc16}},
	{"dataset", []string{"dataset", "active", "-x"}},
	{otctlCmdIPAddr, []string{otctlCmdIPAddr}},
	{otctlCmdNeighborTable, []string{"neighbor", otctlArgTable}},
	{otctlCmdChildTable, []string{"child", otctlArgTable}},
	{otctlCmdRouterTable, []string{"router", otctlArgTable}},
	{otctlCmdCounters, []string{otctlCmdCounters}},
}

func runSnapshotCommands(ctx context.Context, otctl OtCtl) []cmdResult {
	results := make([]cmdResult, len(snapshotCommands))
	var wg sync.WaitGroup
	wg.Add(len(snapshotCommands))
	for i, cmd := range snapshotCommands {
		go func(i int, cmd snapshotCommand) {
			defer wg.Done()
			out, err := otctl.Run(ctx, cmd.args...)
			results[i] = cmdResult{label: cmd.label, value: out, err: err}
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

func assembleSnapshot(values map[string]string, warnings []string) Snapshot {
	var snap Snapshot
	snap.State = values[otctlCmdState]
	snap.NetworkName = values[otctlCmdNetworkName]
	snap.ExtAddress = values[otctlCmdExtAddr]
	snap.PanID = values[otctlCmdPanID]
	snap.Channel = values["channel"]
	snap.Rloc16 = values["rloc16"]
	snap.ActiveDataset = values["dataset"]

	for _, ip := range splitLines(values["ipaddr"]) {
		if ip != "" {
			snap.IPAddresses = append(snap.IPAddresses, ip)
		}
	}

	neighbors := parseNeighborTable(values["neighbor table"])
	children := parseChildTable(values["child table"])
	routers := parseRouterTable(values["router table"])
	snap.Counters = parseCounters(values["counters"])

	gatewayRloc := snap.Rloc16
	if gatewayRloc != "" && !strings.HasPrefix(gatewayRloc, "0x") {
		gatewayRloc = "0x" + gatewayRloc
	}

	snap.Neighbors = mergeMeshNodes(neighbors, children, routers)
	snap.MeshLinks = buildMeshLinks(gatewayRloc, neighbors, children, routers)
	snap.RoutingTree = buildRoutingTree(gatewayRloc, snap.MeshLinks)
	snap.TrafficPaths = buildAllTrafficPaths(gatewayRloc, snap.Neighbors, snap.MeshLinks)
	snap.Warnings = warnings
	return snap
}

// BuildSnapshot collects ot-ctl output and builds the topology snapshot.
func BuildSnapshot(ctx context.Context, otctl OtCtl, opts SnapshotBuildOptions) (Snapshot, error) {
	results := runSnapshotCommands(ctx, otctl)
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

// BuildSnapshotFromTables builds a snapshot from raw ot-ctl table output (for tests).
func BuildSnapshotFromTables(gatewayRloc, rawNeighbors, rawChildren, rawRouters string) Snapshot {
	neighbors := parseNeighborTable(rawNeighbors)
	children := parseChildTable(rawChildren)
	routers := parseRouterTable(rawRouters)
	if gatewayRloc != "" && !strings.HasPrefix(gatewayRloc, "0x") {
		gatewayRloc = "0x" + gatewayRloc
	}
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
