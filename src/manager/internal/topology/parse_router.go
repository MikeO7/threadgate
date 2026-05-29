package topology

import (
	"strconv"
	"strings"
)

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
