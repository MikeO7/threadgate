package topology

import "testing"

func TestFindShortestPath(t *testing.T) {
	adj := map[string][]adjEdge{
		"a": {{key: "b"}, {key: "c"}},
		"b": {{key: "d"}},
		"c": {{key: "d"}},
		"d": {},
	}

	path := findShortestPath("a", "d", adj)
	if len(path) != 3 {
		t.Fatalf("expected 3-hop path, got %v", path)
	}
	if path[0] != "a" || path[len(path)-1] != "d" {
		t.Fatalf("unexpected path: %v", path)
	}
}

func TestFindShortestPathSameNode(t *testing.T) {
	path := findShortestPath("a", "a", nil)
	if len(path) != 1 || path[0] != "a" {
		t.Fatalf("unexpected path: %v", path)
	}
}

func TestFindShortestPathUnreachable(t *testing.T) {
	adj := map[string][]adjEdge{"a": {{key: "b"}}, "b": {}}
	if path := findShortestPath("a", "z", adj); path != nil {
		t.Fatalf("expected nil path, got %v", path)
	}
}

func TestReconstructPath(t *testing.T) {
	previous := map[string]string{"b": "a", "c": "b"}
	path := reconstructPath("a", "c", previous)
	if len(path) != 3 || path[0] != "a" || path[2] != "c" {
		t.Fatalf("unexpected path: %v", path)
	}

	if path := reconstructPath("a", "missing", previous); path != nil {
		t.Fatalf("expected nil for missing goal, got %v", path)
	}
}

func TestBuildTrafficPathUsesBFSFallback(t *testing.T) {
	links := []MeshLink{
		{Kind: linkKindRoute, FromRloc16: "0xc001", ToRloc16: "0xc002"},
		{Kind: linkKindRoute, FromRloc16: "0xc002", ToRloc16: "0xc003"},
	}
	adj := map[string][]adjEdge{
		"c000": {{key: "c001"}},
		"c001": {{key: "c002"}},
		"c002": {{key: "c003"}},
		"c003": {},
	}

	path := buildTrafficPath("0xc000", "0xc003", links, adj)
	if len(path) < 2 {
		t.Fatalf("expected multi-hop path, got %v", path)
	}
}

func TestFindRouteParentLink(t *testing.T) {
	links := []MeshLink{{Kind: linkKindRoute, FromRloc16: "0xc000", ToRloc16: "0xc001"}}
	if link := findRouteParentLink(links, "c001"); link == nil {
		t.Fatal("expected route parent link")
	}
	if link := findRouteParentLink(links, "c999"); link != nil {
		t.Fatal("expected no link")
	}
}

func TestFindDirectGatewayLink(t *testing.T) {
	links := []MeshLink{{Kind: linkKindDirect, FromRloc16: "0xc000", ToRloc16: "0xc001"}}
	if link := findDirectGatewayLink(links, "c000", "c001"); link == nil {
		t.Fatal("expected direct gateway link")
	}
}
