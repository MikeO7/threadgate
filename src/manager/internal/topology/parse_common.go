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
