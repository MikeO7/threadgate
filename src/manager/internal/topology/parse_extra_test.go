package topology

import "testing"

func TestParseNeighborLineFormats(t *testing.T) {
	if n, ok := parseNeighborLine("1122334455667788 0xc001"); ok {
		if n.ExtAddr != "1122334455667788" || n.Rloc16 != "0xc001" {
			t.Fatalf("unexpected neighbor: %+v", n)
		}
	} else {
		t.Fatal("expected simple neighbor parse")
	}

	if _, ok := parseNeighborLine("incomplete"); ok {
		t.Fatal("expected incomplete line to fail")
	}
}

func TestParseCountersFormats(t *testing.T) {
	counters := parseCounters("MacTxUnique=10\nIP: 5\nOther 7\n\n")
	if len(counters) != 3 {
		t.Fatalf("expected 3 counters, got %d", len(counters))
	}
	if counters[0].Key != "MacTxUnique" || counters[0].Val != "10" {
		t.Fatalf("unexpected first counter: %+v", counters[0])
	}
}

func TestRssiToLQI(t *testing.T) {
	if lqi := rssiToLQI("-50"); lqi != 3 {
		t.Fatalf("expected LQI 3 for strong RSSI, got %d", lqi)
	}
	if lqi := rssiToLQI("-90"); lqi != 1 {
		t.Fatalf("expected LQI 1 for weak RSSI, got %d", lqi)
	}
	if lqi := rssiToLQI("invalid"); lqi != 3 {
		t.Fatalf("expected default LQI 3 for invalid RSSI, got %d", lqi)
	}
}

func TestParseMockNeighborLine(t *testing.T) {
	line := "ExtAddr:1122334455667788 Rloc16:0xc001 LinkQuality:3 Role:router"
	n, ok := parseMockNeighborLine(line)
	if !ok || n.Rloc16 != "0xc001" || n.LQI != 3 {
		t.Fatalf("unexpected neighbor: %+v ok=%t", n, ok)
	}
}

func TestParseLeaderData(t *testing.T) {
	raw := `Partition ID: 2271874287
Weighting: 64
Network Data Version: 111
Stable Network Data Version: 112
Leader Router ID: 50
Done`
	ld := parseLeaderData(raw)
	if ld.PartitionID != 2271874287 {
		t.Fatalf("expected PartitionID 2271874287, got %d", ld.PartitionID)
	}
	if ld.Weighting != 64 {
		t.Fatalf("expected Weighting 64, got %d", ld.Weighting)
	}
	if ld.NetworkDataVer != 111 {
		t.Fatalf("expected NetworkDataVer 111, got %d", ld.NetworkDataVer)
	}
	if ld.StableNetworkData != 112 {
		t.Fatalf("expected StableNetworkData 112, got %d", ld.StableNetworkData)
	}
	if ld.LeaderRouterID != 50 {
		t.Fatalf("expected LeaderRouterID 50, got %d", ld.LeaderRouterID)
	}

	// Try with hexadecimal Partition ID and mixed case
	rawHex := `partitionid: 0x87654321
weighting: 32`
	ldHex := parseLeaderData(rawHex)
	if ldHex.PartitionID != 0x87654321 {
		t.Fatalf("expected PartitionID 0x87654321, got 0x%x", ldHex.PartitionID)
	}
	if ldHex.Weighting != 32 {
		t.Fatalf("expected Weighting 32, got %d", ldHex.Weighting)
	}
}

func TestParsePrefixTable(t *testing.T) {
	raw := `Prefixes:
fd00:db8::/64 paros med stable
fd11:2233:4455:1::/64 paos high
Done`
	prefixes := parsePrefixTable(raw)
	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}

	assertPrefixEntry(t, prefixes[0], prefixExpectation{
		prefix:     "fd00:db8::/64",
		preference: prefixPrefMedium,
		stable:     true,
		flag:       "slaac",
	})

	assertPrefixEntry(t, prefixes[1], prefixExpectation{
		prefix:     "fd11:2233:4455:1::/64",
		preference: prefixPrefHigh,
		stable:     true,
	})
}

type prefixExpectation struct {
	prefix     string
	preference string
	stable     bool
	flag       string
}

func assertPrefixEntry(t *testing.T, entry PrefixEntry, want prefixExpectation) {
	t.Helper()
	if entry.Prefix != want.prefix {
		t.Fatalf("unexpected prefix: %s", entry.Prefix)
	}
	if entry.Preference != want.preference {
		t.Fatalf("unexpected preference: %s", entry.Preference)
	}
	if entry.Stable != want.stable {
		t.Fatalf("expected stable=%v", want.stable)
	}
	if want.flag != "" && !containsString(entry.Flags, want.flag) {
		t.Fatalf("expected flags %v to contain %q", entry.Flags, want.flag)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
