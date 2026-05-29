package topology

import (
	"strconv"
	"strings"
)

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
