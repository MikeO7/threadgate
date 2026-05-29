package topology

import "testing"

func TestParseChildTableEmpty(t *testing.T) {
	if len(parseChildTable("")) != 0 {
		t.Fatal("expected empty children list")
	}
}

func TestParseMockChildLine(t *testing.T) {
	line := "ID:1 Rloc16:0x1234 ExtAddr:1122334455667788 LinkQuality:2"
	c, ok := parseMockChildLine(line)
	if !ok || c.ID != 1 || c.Rloc16 != "0x1234" || c.ExtAddr != "1122334455667788" || c.LQI != 2 {
		t.Fatalf("unexpected child: %+v ok=%t", c, ok)
	}

	lineWithoutRloc := "ID:1 ExtAddr:1122334455667788 LinkQuality:2"
	_, ok = parseMockChildLine(lineWithoutRloc)
	if ok {
		t.Fatal("expected false when missing Rloc16")
	}

	invalidFormat := "invalid"
	_, ok = parseMockChildLine(invalidFormat)
	if ok {
		t.Fatal("expected false on invalid format")
	}

	// Test default LQI on missing LinkQuality
	lineWithoutLqi := "ID:1 Rloc16:0x1234 ExtAddr:1122334455667788 LinkQuality:bad"
	c, ok = parseMockChildLine(lineWithoutLqi)
	if !ok || c.LQI != 3 {
		t.Fatalf("expected default LQI 3 on bad format, got %d", c.LQI)
	}
}

func TestParseChildPipeLine(t *testing.T) {
	line := "| 1 | 0x1234 | 1 | 1 | 2 | 1 | 1 | 1122334455667788 |"
	c, ok := parseChildPipeLine(line)
	if !ok || c.ID != 1 || c.Rloc16 != "0x1234" || c.LQI != 2 || c.ExtAddr != "1122334455667788" {
		t.Fatalf("unexpected child: %+v ok=%t", c, ok)
	}

	badLine := "| 1 | 0x1234 |"
	_, ok = parseChildPipeLine(badLine)
	if ok {
		t.Fatal("expected false on bad length line")
	}

	badIDLine := "| bad | 0x1234 | 1 | 1 | 2 | 1 | 1 | 1122334455667788 |"
	_, ok = parseChildPipeLine(badIDLine)
	if ok {
		t.Fatal("expected false on bad ID line")
	}

	badLqLine := "| 1 | 0x1234 | 1 | 1 | bad | 1 | 1 | 1122334455667788 |"
	c, ok = parseChildPipeLine(badLqLine)
	if !ok || c.LQI != 3 {
		t.Fatalf("expected default LQI 3 on bad LQ format, got %d", c.LQI)
	}
}

func TestParseChildTable(t *testing.T) {
	table := `
| ID | RLOC16 | Timeout | Age | LQ In | C_rx | C_tx | Ext Addr |
+----+--------+---------+-----+-------+------+------+------------------+
|  1 | 0x1234 |       1 |   1 |     2 |    1 |    1 | 1122334455667788 |
Rloc16:0x5678 ID:2 ExtAddr:8877665544332211 LinkQuality:1
`
	children := parseChildTable(table)
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}

	if children[0].ID != 1 || children[0].Rloc16 != "0x1234" {
		t.Fatalf("unexpected child 0: %+v", children[0])
	}

	if children[1].ID != 2 || children[1].Rloc16 != "0x5678" {
		t.Fatalf("unexpected child 1: %+v", children[1])
	}
}
