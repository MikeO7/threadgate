package topology

import (
	"strconv"
	"strings"
)

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
