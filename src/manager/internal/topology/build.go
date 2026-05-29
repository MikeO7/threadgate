// Package topology builds mesh graphs and traffic paths from ot-ctl tables.
package topology

import (
	"context"
	"fmt"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

func normalizeGatewayRloc(rloc string) string {
	if rloc == "" {
		return ""
	}
	if !strings.HasPrefix(rloc, "0x") {
		return "0x" + rloc
	}
	return rloc
}

// AssembleSnapshot builds a topology model from collected ot-ctl label values.
func AssembleSnapshot(values map[string]string, warnings []string) Snapshot {
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
	policy := opts.Policy
	if policy != otctl.PolicyStrict {
		policy = otctl.PolicyBestEffort
	}
	values, warnings, err := otctl.CollectParallel(ctx, runner, otctl.SnapshotCommands, policy)
	snap := AssembleSnapshot(values, warnings)
	if len(warnings) > 0 && policy == otctl.PolicyBestEffort && err != nil {
		return snap, fmt.Errorf("snapshot partial: %s", warnings[0])
	}
	return snap, err
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
