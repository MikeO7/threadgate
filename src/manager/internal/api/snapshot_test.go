package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNeighborPipeTableGolden(t *testing.T) {
	raw := readFixture(t, "neighbor_table.txt")
	neighbors := parseNeighborTable(raw)
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(neighbors))
	}
	if neighbors[0].Rloc16 != "0xc001" || neighbors[0].LQI != 3 {
		t.Fatalf("unexpected first neighbor: %+v", neighbors[0])
	}
	if neighbors[1].LQI != 2 {
		t.Fatalf("expected LQI 2 from RSSI -68, got %d", neighbors[1].LQI)
	}
}

func TestParseChildPipeTableGolden(t *testing.T) {
	raw := readFixture(t, "child_table.txt")
	children := parseChildTable(raw)
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	if children[0].ID != 27 || children[0].ExtAddr != "112233445566771b" {
		t.Fatalf("unexpected child: %+v", children[0])
	}
}

func TestParseRouterPipeTableGolden(t *testing.T) {
	raw := readFixture(t, "router_table.txt")
	routers := parseRouterTable(raw)
	if len(routers) != 3 {
		t.Fatalf("expected 3 routers, got %d", len(routers))
	}
	if routers[2].NextHopID != 2 || routers[2].PathCost != 2 {
		t.Fatalf("unexpected router: %+v", routers[2])
	}
}

func TestBuildSnapshotFromTablesGolden(t *testing.T) {
	snap := BuildSnapshotFromTables(
		"0xc000",
		readFixture(t, "neighbor_table.txt"),
		readFixture(t, "child_table.txt"),
		readFixture(t, "router_table.txt"),
	)
	if len(snap.Neighbors) < 4 {
		t.Fatalf("expected merged nodes, got %d", len(snap.Neighbors))
	}
	if len(snap.MeshLinks) == 0 {
		t.Fatal("expected mesh links")
	}
	if len(snap.TrafficPaths) == 0 {
		t.Fatal("expected traffic paths")
	}
	if snap.RoutingTree.GatewayKey != testGatewayKey {
		t.Fatalf("unexpected gateway key: %q", snap.RoutingTree.GatewayKey)
	}
}

func TestBuildTrafficPathGolden(t *testing.T) {
	snap := BuildSnapshotFromTables(
		"0xc000",
		readFixture(t, "neighbor_table.txt"),
		readFixture(t, "child_table.txt"),
		readFixture(t, "router_table.txt"),
	)
	path, ok := snap.TrafficPaths[normalizeRloc16("0xc003")]
	if !ok {
		t.Fatal("expected traffic path for 0xc003")
	}
	if len(path) < 2 {
		t.Fatalf("expected multi-hop path, got %v", path)
	}
	if normalizeRloc16(path[0]) != testGatewayKey {
		t.Fatalf("path should start at gateway, got %v", path)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	//nolint:gosec // G304: reads fixed test fixture names under testdata/
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}
