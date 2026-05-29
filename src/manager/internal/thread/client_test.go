package thread

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

const testGatewayKey = "c000"

func TestClientBuildSnapshot(t *testing.T) {
	runner := FuncRunner(snapshotFixtureOtCtl(t))
	client := NewClient(runner, PolicyBestEffort)

	snap, err := client.BuildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if len(snap.Neighbors) < 4 {
		t.Fatalf("expected merged mesh nodes, got %d", len(snap.Neighbors))
	}
	if len(snap.MeshLinks) == 0 {
		t.Fatal("expected mesh links")
	}
	if len(snap.TrafficPaths) == 0 {
		t.Fatal("expected traffic paths")
	}
	if snap.RoutingTree.GatewayKey != testGatewayKey {
		t.Fatalf("unexpected gateway key: %q", snap.RoutingTree.GatewayKey)
	}
}

func TestClientBuildSnapshotPartialFailure(t *testing.T) {
	runner := FuncRunner(func(_ context.Context, args ...string) (string, error) {
		key := otctl.Command{Args: args}.Key()
		if key == otctl.State.Key() {
			return "", context.Canceled
		}
		return "ok", nil
	})
	client := NewClient(runner, PolicyBestEffort)

	snap, err := client.BuildSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected best-effort partial snapshot error")
	}
	if len(snap.Warnings) == 0 {
		t.Fatal("expected warnings on partial snapshot")
	}
}

func TestClientBuildSnapshotStrictMode(t *testing.T) {
	runner := FuncRunner(func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == otctl.State.Args[0] {
			return "", context.Canceled
		}
		return "ok", nil
	})
	client := NewClient(runner, PolicyStrict)

	_, err := client.BuildSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected strict collection to fail on first error")
	}
}

func snapshotFixtureOtCtl(t *testing.T) func(context.Context, ...string) (string, error) {
	t.Helper()
	fixtures := map[string]string{
		otctl.State.Key():         mockLeader,
		otctl.NetworkName.Key():   TestNetworkName,
		otctl.ExtAddr.Key():       "1122334455667788",
		otctl.PanID.Key():         "0x1234",
		otctl.Channel.Key():       "15",
		otctl.Rloc16.Key():        "0xc000",
		otctl.DatasetActive.Key(): MockActiveDataset,
		otctl.IPAddr.Key():        "fd00::1",
		otctl.Counters.Key():      "MacTxUnique=1",
		otctl.NeighborTable.Key(): readFixture(t, "neighbor_table.txt"),
		otctl.ChildTable.Key():    readFixture(t, "child_table.txt"),
		otctl.RouterTable.Key():   readFixture(t, "router_table.txt"),
		otctl.LeaderData.Key():    "Partition ID: 2271874287\nWeighting: 64\nNetwork Data Version: 111\nStable Network Data Version: 112\nLeader Router ID: 50\nDone",
		otctl.PrefixTable.Key():   "fd11:22::/64 paros med stable\nDone",
	}

	return func(_ context.Context, args ...string) (string, error) {
		key := otctl.Command{Args: args}.Key()
		if val, ok := fixtures[key]; ok {
			return val, nil
		}
		return "", fmt.Errorf("unknown command: %s", key)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	//nolint:gosec // G304: reads fixed test fixture names under testdata/
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestScanChannels(t *testing.T) {
	mockOutput := `| Ch | RSSI |
+----+------+
| 11 |  -82 |
| 15 |  -92 |
| 20 |  -70 |
| 26 |  -55 |
Done`

	runner := FuncRunner(func(_ context.Context, args ...string) (string, error) {
		key := otctl.Command{Args: args}.Key()
		if key == otctl.ScanEnergy.Key() {
			return mockOutput, nil
		}
		return "", fmt.Errorf("unknown command: %s", key)
	})

	client := NewClient(runner, PolicyBestEffort)
	results, err := client.ScanChannels(context.Background())
	if err != nil {
		t.Fatalf("ScanChannels failed: %v", err)
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	assertChannelScanResult(t, results[0], 11, -82, RatingGood)
	assertChannelScanResult(t, results[1], 15, -92, RatingExcellent)
	assertChannelScanResult(t, results[2], 20, -70, RatingFair)
	assertChannelScanResult(t, results[3], 26, -55, RatingPoor)
}

func assertChannelScanResult(t *testing.T, got ChannelScanResult, wantChannel, wantRSSI int, wantRating string) {
	t.Helper()
	if got.Channel != wantChannel || got.RSSI != wantRSSI || got.Rating != wantRating {
		t.Errorf("channel scan result = %+v, want channel=%d rssi=%d rating=%q", got, wantChannel, wantRSSI, wantRating)
	}
}
