package api

import (
	"context"
	"strings"
	"testing"
)

func TestThreadServiceBuildSnapshot(t *testing.T) {
	otctl := FuncOtCtl(snapshotFixtureOtCtl(t))
	svc := NewThreadService(otctl, CollectBestEffort)

	snap, err := svc.BuildSnapshot(context.Background())
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

func TestBuildSnapshotStrictMode(t *testing.T) {
	otctl := FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == otctlCmdState {
			return "", context.Canceled
		}
		return "ok", nil
	})

	_, err := BuildSnapshot(context.Background(), otctl, SnapshotBuildOptions{Mode: CollectStrict})
	if err == nil {
		t.Fatal("expected strict collection to fail on first error")
	}
}

func snapshotFixtureOtCtl(t *testing.T) func(context.Context, ...string) (string, error) {
	t.Helper()
	fixtures := map[string]string{
		otctlCmdState:             "leader",
		otctlCmdNetworkName:       testNetworkName,
		otctlCmdExtAddr:           "1122334455667788",
		otctlCmdPanID:             "0x1234",
		otctlCmdChannel:          "15",
		otctlCmdRloc16:           "0xc000",
		otctlCmdDatasetActiveX:   activeDatasetHex,
		otctlCmdIPAddr:           "fd00::1",
		otctlCmdCounters:         "MacTxUnique=1",
		otctlCmdNeighborTable:    readFixture(t, "neighbor_table.txt"),
		otctlCmdChildTable:       readFixture(t, "child_table.txt"),
		otctlCmdRouterTable:      readFixture(t, "router_table.txt"),
	}

	return func(_ context.Context, args ...string) (string, error) {
		key := strings.Join(args, " ")
		if val, ok := fixtures[key]; ok {
			return val, nil
		}
		return "", context.Canceled
	}
}
