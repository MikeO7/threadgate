package topology

import (
	"strconv"
	"strings"
)

func normalizeRloc16(rloc string) string {
	rloc = strings.TrimSpace(strings.ToLower(rloc))
	rloc = strings.TrimPrefix(rloc, "0x")
	return rloc
}

func isTableHeaderLine(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "rloc") && strings.Contains(lower, "extended")
}

func rssiToLQI(rssiField string) int {
	rssi, err := strconv.Atoi(strings.TrimSpace(rssiField))
	if err != nil {
		return 3
	}
	switch {
	case rssi >= -60:
		return 3
	case rssi >= -75:
		return 2
	default:
		return 1
	}
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	var list []string
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func parseChildTable(output string) []ChildEntry {
	var children []ChildEntry
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "---") || isTableHeaderLine(line) {
			continue
		}
		if strings.Contains(line, "Rloc16:") {
			if c, ok := parseMockChildLine(line); ok {
				children = append(children, c)
			}
			continue
		}
		if c, ok := parseChildPipeLine(line); ok {
			children = append(children, c)
		}
	}
	return children
}

func parseMockChildLine(line string) (ChildEntry, bool) {
	parts := strings.Fields(line)
	var c ChildEntry
	for _, part := range parts {
		sub := strings.SplitN(part, ":", 2)
		if len(sub) != 2 {
			continue
		}
		switch sub[0] {
		case "ID":
			c.ID, _ = strconv.Atoi(sub[1])
		case keyRloc16:
			c.Rloc16 = sub[1]
		case keyExtAddr:
			c.ExtAddr = sub[1]
		case keyLinkQuality:
			c.LQI = 3
			if val, err := strconv.Atoi(sub[1]); err == nil {
				c.LQI = val
			}
		}
	}
	return c, c.Rloc16 != ""
}

func parseChildPipeLine(line string) (ChildEntry, bool) {
	line = strings.Trim(line, "|")
	fields := strings.Split(line, "|")
	if len(fields) < 8 {
		return ChildEntry{}, false
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil {
		return ChildEntry{}, false
	}
	lq := 3
	if val, err := strconv.Atoi(fields[4]); err == nil {
		lq = val
	}
	return ChildEntry{ID: id, Rloc16: fields[1], LQI: lq, ExtAddr: fields[7]}, true
}

func parseNeighborTable(output string) []Neighbor {
	var neighbors []Neighbor
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "---") || isTableHeaderLine(line) {
			continue
		}
		if n, ok := parseNeighborLine(line); ok {
			neighbors = append(neighbors, n)
		}
	}
	return neighbors
}

func parseNeighborLine(line string) (Neighbor, bool) {
	if strings.Contains(line, "ExtAddr:") {
		return parseMockNeighborLine(line)
	}
	if strings.Contains(line, "|") {
		return parseNeighborPipeLine(line)
	}
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		return Neighbor{ExtAddr: parts[0], Rloc16: parts[1], LQI: 3}, true
	}
	return Neighbor{}, false
}

func parseNeighborPipeLine(line string) (Neighbor, bool) {
	line = strings.Trim(line, "|")
	fields := strings.Split(line, "|")
	if len(fields) < 9 {
		return Neighbor{}, false
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return Neighbor{
		Role:    fields[0],
		Rloc16:  fields[1],
		ExtAddr: fields[len(fields)-1],
		LQI:     rssiToLQI(fields[4]),
	}, true
}

func parseMockNeighborLine(line string) (Neighbor, bool) {
	parts := strings.Fields(line)
	var n Neighbor
	for _, part := range parts {
		sub := strings.SplitN(part, ":", 2)
		if len(sub) != 2 {
			continue
		}
		switch sub[0] {
		case keyExtAddr:
			n.ExtAddr = sub[1]
		case keyRloc16:
			n.Rloc16 = sub[1]
		case keyLinkQuality:
			n.LQI = 3
			if val, err := strconv.Atoi(sub[1]); err == nil {
				n.LQI = val
			}
		case "Role":
			n.Role = sub[1]
		}
	}
	return n, n.ExtAddr != ""
}

func parseRouterTable(output string) []RouterEntry {
	var routers []RouterEntry
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "---") || isTableHeaderLine(line) {
			continue
		}
		if strings.Contains(line, "Rloc16:") {
			if r, ok := parseMockRouterLine(line); ok {
				routers = append(routers, r)
			}
			continue
		}
		if r, ok := parseRouterPipeLine(line); ok {
			routers = append(routers, r)
		}
	}
	return routers
}

func parseMockRouterLine(line string) (RouterEntry, bool) {
	parts := strings.Fields(line)
	var r RouterEntry
	for _, part := range parts {
		sub := strings.SplitN(part, ":", 2)
		if len(sub) != 2 {
			continue
		}
		switch sub[0] {
		case "ID":
			r.ID, _ = strconv.Atoi(sub[1])
		case keyRloc16:
			r.Rloc16 = sub[1]
		case "NextHop":
			r.NextHopID, _ = strconv.Atoi(sub[1])
		case "PathCost":
			r.PathCost, _ = strconv.Atoi(sub[1])
		case keyExtAddr:
			r.ExtAddr = sub[1]
		case keyLinkQuality:
			r.LinkQuality = 3
			if val, err := strconv.Atoi(sub[1]); err == nil {
				r.LinkQuality = val
			}
		}
	}
	return r, r.Rloc16 != ""
}

func parseRouterPipeLine(line string) (RouterEntry, bool) {
	line = strings.Trim(line, "|")
	fields := strings.Split(line, "|")
	if len(fields) < 7 {
		return RouterEntry{}, false
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	id, err := strconv.Atoi(fields[0])
	if err != nil {
		return RouterEntry{}, false
	}
	nextHop, err := strconv.Atoi(fields[2])
	if err != nil {
		return RouterEntry{}, false
	}
	pathCost, err := strconv.Atoi(fields[3])
	if err != nil {
		return RouterEntry{}, false
	}
	lq := 3
	if val, err := strconv.Atoi(fields[4]); err == nil {
		lq = val
	}
	extAddr := fields[6]
	if len(fields) > 7 {
		extAddr = fields[len(fields)-1]
	}
	return RouterEntry{
		ID: id, Rloc16: fields[1], NextHopID: nextHop, PathCost: pathCost,
		ExtAddr: extAddr, LinkQuality: lq,
	}, true
}

const (
	prefixPrefHigh   = "high"
	prefixPrefMedium = "medium"
	prefixPrefLow    = "low"
)

func parseCounters(output string) []Counter {
	var list []Counter
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var key, val string
		switch {
		case strings.Contains(line, "="):
			parts := strings.SplitN(line, "=", 2)
			key, val = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		case strings.Contains(line, ":"):
			parts := strings.SplitN(line, ":", 2)
			key, val = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		default:
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key, val = parts[0], parts[1]
			}
		}
		if key != "" && val != "" {
			list = append(list, Counter{Key: key, Val: val})
		}
	}
	return list
}

func parseLeaderData(output string) LeaderData {
	var ld LeaderData
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "Done" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		applyLeaderField(&ld, strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1]))
	}
	return ld
}

func applyLeaderField(ld *LeaderData, key, valStr string) {
	switch key {
	case "partition id", "partitionid":
		if u, err := strconv.ParseUint(valStr, 10, 32); err == nil {
			ld.PartitionID = uint32(u)
		} else if strings.HasPrefix(strings.ToLower(valStr), "0x") {
			if u, err := strconv.ParseUint(valStr[2:], 16, 32); err == nil {
				ld.PartitionID = uint32(u)
			}
		}
	case "weighting":
		ld.Weighting, _ = strconv.Atoi(valStr)
	case "network data version", "networkdataversion", "network data ver":
		ld.NetworkDataVer, _ = strconv.Atoi(valStr)
	case "stable network data version", "stablenetworkdataversion", "stable network data ver":
		ld.StableNetworkData, _ = strconv.Atoi(valStr)
	case "leader router id", "leaderrouterid":
		ld.LeaderRouterID, _ = strconv.Atoi(valStr)
	}
}

func parsePrefixTable(output string) []PrefixEntry {
	var prefixes []PrefixEntry
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "Done" || strings.ToLower(line) == "prefixes:" {
			continue
		}
		if entry, ok := parsePrefixLine(line); ok {
			prefixes = append(prefixes, entry)
		}
	}
	return prefixes
}

func parsePrefixLine(line string) (PrefixEntry, bool) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return PrefixEntry{}, false
	}
	prefix := parts[0]
	if !strings.Contains(prefix, "/") || !strings.Contains(prefix, ":") {
		return PrefixEntry{}, false
	}

	flags, preference, stable := parsePrefixAttributes(parts[1:])
	flags = uniqueStrings(flags)

	return PrefixEntry{
		Prefix:     prefix,
		Flags:      flags,
		Stable:     stable,
		Preference: preference,
	}, true
}

func parsePrefixAttributes(parts []string) (flags []string, preference string, stable bool) {
	preference = prefixPrefMedium
	for _, part := range parts {
		partLower := strings.ToLower(part)
		switch partLower {
		case prefixPrefHigh:
			preference = prefixPrefHigh
		case "med", prefixPrefMedium:
			preference = prefixPrefMedium
		case prefixPrefLow:
			preference = prefixPrefLow
		case "stable":
			stable = true
		default:
			flags, stable = appendPrefixFlag(flags, part, partLower, stable)
		}
	}
	return flags, preference, stable
}

func appendPrefixFlag(flags []string, part, partLower string, stable bool) ([]string, bool) {
	if len(part) > 6 || strings.Contains(part, ":") {
		return append(flags, part), stable
	}
	for _, char := range partLower {
		flag, ok := prefixFlagForChar(char)
		if !ok {
			continue
		}
		flags = append(flags, flag)
		if char == 's' {
			stable = true
		}
	}
	return flags, stable
}

func prefixFlagForChar(char rune) (string, bool) {
	switch char {
	case 'p':
		return "preferred", true
	case 'a':
		return "slaac", true
	case 'd':
		return "dhcp", true
	case 'c':
		return "config", true
	case 'r':
		return "route", true
	case 'o':
		return "on-mesh", true
	case 's':
		return "stable", true
	default:
		return "", false
	}
}
