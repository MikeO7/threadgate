package api

func findRouteParentLink(links []MeshLink, currentKey string) *MeshLink {
	for i := range links {
		link := links[i]
		if link.Kind == linkKindRoute && normalizeRloc16(link.ToRloc16) == currentKey {
			return &link
		}
	}
	return nil
}

func findDirectGatewayLink(links []MeshLink, gatewayKey, currentKey string) *MeshLink {
	for i := range links {
		link := links[i]
		if link.Kind != linkKindDirect && link.Kind != linkKindChild {
			continue
		}
		if normalizeRloc16(link.FromRloc16) == gatewayKey && normalizeRloc16(link.ToRloc16) == currentKey {
			return &link
		}
	}
	return nil
}

func buildTrafficPath(gatewayRloc, targetRloc string, links []MeshLink, adj map[string][]adjEdge) []string {
	gatewayKey := normalizeRloc16(gatewayRloc)
	targetKey := normalizeRloc16(targetRloc)
	if targetKey == gatewayKey {
		return []string{gatewayRloc}
	}

	path := []string{targetRloc}
	currentKey := targetKey

	for guard := 0; currentKey != gatewayKey && guard < 32; guard++ {
		if parentLink := findRouteParentLink(links, currentKey); parentLink != nil {
			path = append([]string{parentLink.FromRloc16}, path...)
			currentKey = normalizeRloc16(parentLink.FromRloc16)
			continue
		}
		if oneHop := findDirectGatewayLink(links, gatewayKey, currentKey); oneHop != nil {
			path = append([]string{gatewayRloc}, path...)
			currentKey = gatewayKey
			continue
		}
		break
	}

	if normalizeRloc16(path[0]) != gatewayKey {
		if bfs := findShortestPath(gatewayKey, targetKey, adj); len(bfs) >= 2 {
			return bfs
		}
		path = append([]string{gatewayRloc}, path...)
	}
	return path
}

func reconstructPath(startKey, goalKey string, previous map[string]string) []string {
	if previous[goalKey] == "" && startKey != goalKey {
		return nil
	}
	chain := []string{goalKey}
	cursor := goalKey
	for cursor != startKey {
		cursor = previous[cursor]
		if cursor == "" {
			return nil
		}
		chain = append([]string{cursor}, chain...)
	}
	return chain
}

func findShortestPath(startKey, goalKey string, adj map[string][]adjEdge) []string {
	if startKey == goalKey {
		return []string{startKey}
	}
	queue := []string{startKey}
	visited := map[string]bool{startKey: true}
	previous := make(map[string]string)

	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		for _, edge := range adj[key] {
			nextKey := edge.key
			if visited[nextKey] {
				continue
			}
			visited[nextKey] = true
			previous[nextKey] = key
			if nextKey == goalKey {
				queue = nil
				break
			}
			queue = append(queue, nextKey)
		}
	}

	if !visited[goalKey] {
		return nil
	}
	return reconstructPath(startKey, goalKey, previous)
}
