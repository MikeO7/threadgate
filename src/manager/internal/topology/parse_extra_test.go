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
