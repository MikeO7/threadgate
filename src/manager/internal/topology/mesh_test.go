package topology

import "testing"

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
