package topology

import (
	"sort"
)

func mergeMeshNodes(neighbors []Neighbor, children []ChildEntry, routers []RouterEntry) []Neighbor {
	byRloc := make(map[string]Neighbor)
	addNeighbors(byRloc, neighbors)
	addChildren(byRloc, children)
	addRouters(byRloc, routers)
	if len(byRloc) == 0 {
		return neighbors
	}

	// ⚡ Bolt: Use a Schwartzian transform to cache normalized Rloc16 strings for sorting.
	// This avoids expensive normalizeRloc16 calls on every sort comparison (O(N log N))
	// while remaining strictly faithful to the original data source.
	type sortNode struct {
		norm string
		node Neighbor
	}
	nodes := make([]sortNode, 0, len(byRloc))
	for _, n := range byRloc {
		nodes = append(nodes, sortNode{norm: normalizeRloc16(n.Rloc16), node: n})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].norm < nodes[j].norm
	})

	out := make([]Neighbor, len(nodes))
	for i, sn := range nodes {
		out[i] = sn.node
	}
	return out
}

func addMeshNode(byRloc map[string]Neighbor, rloc, ext string, lqi, pathCost int, nextHop, role string) {
	key := normalizeRloc16(rloc)
	if key == "" {
		return
	}
	current := byRloc[key]
	if ext != "" {
		current.ExtAddr = ext
	}
	if rloc != "" {
		current.Rloc16 = rloc
	}
	if lqi > 0 {
		current.LQI = lqi
	}
	if pathCost >= 0 {
		current.PathCost = pathCost
	}
	if nextHop != "" {
		current.NextHopRloc = nextHop
	}
	if role != "" {
		current.Role = role
	}
	if current.LQI == 0 {
		current.LQI = 3
	}
	byRloc[key] = current
}

func addNeighbors(byRloc map[string]Neighbor, neighbors []Neighbor) {
	for _, n := range neighbors {
		addMeshNode(byRloc, n.Rloc16, n.ExtAddr, n.LQI, n.PathCost, n.NextHopRloc, n.Role)
	}
}

func addChildren(byRloc map[string]Neighbor, children []ChildEntry) {
	for _, c := range children {
		addMeshNode(byRloc, c.Rloc16, c.ExtAddr, c.LQI, 1, "", "child")
	}
}

func addRouters(byRloc map[string]Neighbor, routers []RouterEntry) {
	idToRloc := make(map[int]string, len(routers))
	for _, r := range routers {
		idToRloc[r.ID] = r.Rloc16
	}
	for _, r := range routers {
		nextHop := ""
		if hopRloc, ok := idToRloc[r.NextHopID]; ok {
			nextHop = hopRloc
		}
		addMeshNode(byRloc, r.Rloc16, r.ExtAddr, r.LinkQuality, r.PathCost, nextHop, "router")
	}
}

type linkBuilder struct {
	links []MeshLink
	seen  map[string]struct{}
}

func newLinkBuilder(capacity int) *linkBuilder {
	return &linkBuilder{links: make([]MeshLink, 0, capacity), seen: make(map[string]struct{})}
}

func (b *linkBuilder) add(from, to, kind string, pathCost int) {
	if from == "" || to == "" {
		return
	}
	x := normalizeRloc16(from)
	y := normalizeRloc16(to)
	if x == y {
		return
	}
	key := kind + "|" + x + "|" + y
	rev := kind + "|" + y + "|" + x
	if _, ok := b.seen[key]; ok {
		return
	}
	if kind != linkKindRoute {
		if _, ok := b.seen[rev]; ok {
			return
		}
	}
	b.seen[key] = struct{}{}
	b.links = append(b.links, MeshLink{FromRloc16: from, ToRloc16: to, Kind: kind, PathCost: pathCost})
}

func buildMeshLinks(gatewayRloc string, neighbors []Neighbor, children []ChildEntry, routers []RouterEntry) []MeshLink {
	builder := newLinkBuilder(len(neighbors) + len(children) + len(routers))
	for _, n := range neighbors {
		builder.add(gatewayRloc, n.Rloc16, linkKindDirect, 1)
	}
	for _, c := range children {
		builder.add(gatewayRloc, c.Rloc16, linkKindChild, 1)
	}
	buildRouterLinks(builder, routers)
	return builder.links
}

func buildRouterLinks(builder *linkBuilder, routers []RouterEntry) {
	idToRloc := make(map[int]string, len(routers))
	for _, r := range routers {
		idToRloc[r.ID] = r.Rloc16
	}
	for _, r := range routers {
		if r.NextHopID == r.ID || r.PathCost <= 0 {
			continue
		}
		fromRloc, ok := idToRloc[r.NextHopID]
		if !ok {
			continue
		}
		builder.add(fromRloc, r.Rloc16, linkKindRoute, r.PathCost)
	}
}

func applyRouteLinks(parentOf map[string]RoutingParentEntry, links []MeshLink) {
	for _, link := range links {
		if link.Kind != linkKindRoute {
			continue
		}
		fromKey := normalizeRloc16(link.FromRloc16)
		toKey := normalizeRloc16(link.ToRloc16)
		parentOf[toKey] = RoutingParentEntry{Parent: fromKey, Link: link}
	}
}

func applyDirectChildLinks(parentOf map[string]RoutingParentEntry, gatewayKey string, links []MeshLink) {
	for _, link := range links {
		if link.Kind != linkKindDirect && link.Kind != linkKindChild {
			continue
		}
		fromKey := normalizeRloc16(link.FromRloc16)
		toKey := normalizeRloc16(link.ToRloc16)
		if fromKey != gatewayKey {
			continue
		}
		if _, exists := parentOf[toKey]; exists {
			continue
		}
		parentOf[toKey] = RoutingParentEntry{Parent: fromKey, Link: link}
	}
}

func buildChildrenOf(parentOf map[string]RoutingParentEntry) map[string][]string {
	childrenOf := make(map[string][]string)
	for childKey, entry := range parentOf {
		parentKey := entry.Parent
		childrenOf[parentKey] = append(childrenOf[parentKey], childKey)
	}
	for parentKey := range childrenOf {
		sort.Strings(childrenOf[parentKey])
	}
	return childrenOf
}

func buildRoutingTree(gatewayRloc string, links []MeshLink) RoutingTree {
	gatewayKey := normalizeRloc16(gatewayRloc)
	parentOf := make(map[string]RoutingParentEntry)
	applyRouteLinks(parentOf, links)
	applyDirectChildLinks(parentOf, gatewayKey, links)
	return RoutingTree{
		ParentOf:   parentOf,
		ChildrenOf: buildChildrenOf(parentOf),
		GatewayKey: gatewayKey,
	}
}

func buildAllTrafficPaths(gatewayRloc string, nodes []Neighbor, links []MeshLink) map[string][]string {
	gatewayKey := normalizeRloc16(gatewayRloc)
	paths := make(map[string][]string, len(nodes))
	adj := buildAdjacency(links)

	for _, n := range nodes {
		targetKey := normalizeRloc16(n.Rloc16)
		if targetKey == "" || targetKey == gatewayKey {
			continue
		}
		path := buildTrafficPath(gatewayRloc, n.Rloc16, links, adj)
		paths[targetKey] = append([]string(nil), path...)
	}
	return paths
}

func buildAdjacency(links []MeshLink) map[string][]adjEdge {
	adj := make(map[string][]adjEdge)
	for _, link := range links {
		fromKey := normalizeRloc16(link.FromRloc16)
		toKey := normalizeRloc16(link.ToRloc16)
		adj[fromKey] = append(adj[fromKey], adjEdge{key: toKey, link: link})
		adj[toKey] = append(adj[toKey], adjEdge{key: fromKey, link: link})
	}
	return adj
}

type adjEdge struct {
	key  string
	link MeshLink
}
