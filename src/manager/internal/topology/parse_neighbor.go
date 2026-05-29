package topology

import (
	"strconv"
	"strings"
)

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
