package thread

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestDecodeDataset(t *testing.T) {
	// A valid hex operational dataset with standard TLVs:
	// Type 0 (Channel): Length 3 -> 00 03 00 00 0f (Channel 15)
	// Type 1 (PAN ID): Length 2 -> 01 02 12 34 (PAN ID 0x1234)
	// Type 2 (Extended PAN ID): Length 8 -> 02 08 11 22 33 44 55 66 77 88
	// Type 3 (Network Name): Length 11 -> 03 0b 54 68 72 65 61 64 2d 54 65 73 74 (Thread-Test)
	// Type 5 (Network Key): Length 16 -> mock key (not the OpenThread web UI default)
	// Type 7 (Mesh-Local Prefix): Length 8 -> 07 08 fd 00 0d b8 00 00 00 00
	// Type 14 (Active Timestamp): Length 8 -> 0e 08 00 00 00 00 00 00 00 01
	hexDataset := ValidOperationalDatasetHex

	decoded, err := DecodeDataset(hexDataset)
	if err != nil {
		t.Fatalf("DecodeDataset failed: %v", err)
	}

	if decoded.NetworkName != TestNetworkName {
		t.Errorf("expected NetworkName %q, got %q", TestNetworkName, decoded.NetworkName)
	}
	if decoded.Channel != 15 {
		t.Errorf("expected Channel 15, got %d", decoded.Channel)
	}
	if decoded.PanID != "0x1234" {
		t.Errorf("expected PanID '0x1234', got %q", decoded.PanID)
	}
	if decoded.ExtPanID != "1122334455667788" {
		t.Errorf("expected ExtPanID '1122334455667788', got %q", decoded.ExtPanID)
	}
	if decoded.NetworkKey != MockNetworkKeyHex {
		t.Errorf("expected NetworkKey %q, got %q", MockNetworkKeyHex, decoded.NetworkKey)
	}
	if decoded.MeshLocalPrefix != "fd00:0db8:0000:0000::/64" {
		t.Errorf("expected MeshLocalPrefix 'fd00:0db8:0000:0000::/64', got %q", decoded.MeshLocalPrefix)
	}
	if decoded.ActiveTimestamp != 2 {
		t.Errorf("expected ActiveTimestamp 2, got %d", decoded.ActiveTimestamp)
	}
}

func TestCheckHealth(t *testing.T) {
	// Setup a mock runner for successful case
	fixtures := map[string]string{
		"state":          mockLeader,
		"networkname":    TestNetworkName,
		"extaddr":        "1122334455667788",
		"panid":          "0x1234",
		"rloc16":         "0xc000",
		"ipaddr":         "fd00::1",
		"counters":       "MacTxUnique=1",
		"neighbor table": "rloc16 | extaddr | rssi\n0xc001 | 2233445566778899 | -50\nDone",
	}

	runner := FuncRunner(func(_ context.Context, args ...string) (string, error) {
		if len(args) == 0 {
			return "", fmt.Errorf("empty args")
		}
		cmd := strings.Join(args, " ")
		if val, ok := fixtures[cmd]; ok {
			return val, nil
		}
		return "", fmt.Errorf("unknown command: %s", cmd)
	})

	client := NewClient(runner, PolicyBestEffort)
	summary, err := client.CheckHealth(context.Background())
	if err != nil {
		t.Fatalf("CheckHealth failed: %v", err)
	}

	if !summary.Healthy {
		t.Error("expected healthy summary")
	}
	if summary.State != mockLeader {
		t.Errorf("expected state %q, got %q", mockLeader, summary.State)
	}
	if summary.NeighborCount != 1 {
		t.Errorf("expected neighbor count 1, got %d", summary.NeighborCount)
	}
}
