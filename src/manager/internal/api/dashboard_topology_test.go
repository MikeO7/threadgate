package api

import (
	"context"
	"testing"
)

func TestBuildMeshLinks(t *testing.T) {
	gateway := "0xc000"
	neighbors := []Neighbor{
		{Rloc16: "0xc001", ExtAddr: "1", LQI: 3},
		{Rloc16: "0xc002", ExtAddr: "2", LQI: 2},
	}
	children := []ChildEntry{
		{Rloc16: "0xc01e", ExtAddr: "30", LQI: 2},
	}
	routers := []RouterEntry{
		{ID: 1, Rloc16: "0xc001", NextHopID: 1, PathCost: 0},
		{ID: 2, Rloc16: "0xc002", NextHopID: 1, PathCost: 1},
		{ID: 3, Rloc16: "0xc003", NextHopID: 2, PathCost: 2},
	}

	links := buildMeshLinks(gateway, neighbors, children, routers)

	counts := map[string]int{}
	for _, link := range links {
		counts[link.Kind]++
	}

	if counts[linkKindDirect] != 2 {
		t.Fatalf("expected 2 direct links, got %d", counts[linkKindDirect])
	}
	if counts[linkKindChild] != 1 {
		t.Fatalf("expected 1 child link, got %d", counts[linkKindChild])
	}
	if counts[linkKindRoute] < 2 {
		t.Fatalf("expected route links, got %d", counts[linkKindRoute])
	}
}

func TestParseRouterTableMock(t *testing.T) {
	output := "ID:3 Rloc16:0xc003 NextHop:2 PathCost:2 ExtAddr:0000000000000003 LinkQuality:2"
	routers := parseRouterTable(output)
	if len(routers) != 1 {
		t.Fatalf("expected 1 router, got %d", len(routers))
	}
	if routers[0].NextHopID != 2 || routers[0].PathCost != 2 {
		t.Fatalf("unexpected router entry: %+v", routers[0])
	}
}

func TestMockTopologyConsistency(t *testing.T) {
	otctl := NewMockOtCtl()
	ctx := context.Background()

	neighbors := parseNeighborTable(mustRunOtCtl(ctx, t, otctl, "neighbor", "table"))
	children := parseChildTable(mustRunOtCtl(ctx, t, otctl, "child", "table"))
	routers := parseRouterTable(mustRunOtCtl(ctx, t, otctl, "router", "table"))
	links := buildMeshLinks("0xc000", neighbors, children, routers)

	if len(neighbors) != mockDirectCount {
		t.Fatalf("expected %d direct neighbors, got %d", mockDirectCount, len(neighbors))
	}
	if len(routers) != mockNodeCount {
		t.Fatalf("expected %d routers, got %d", mockNodeCount, len(routers))
	}
	if len(links) == 0 {
		t.Fatal("expected topology links")
	}
}

func mustRunOtCtl(ctx context.Context, t *testing.T, otctl OtCtl, args ...string) string {
	t.Helper()
	out, err := otctl.Run(ctx, args...)
	if err != nil {
		t.Fatalf("otctl failed: %v", err)
	}
	return out
}
