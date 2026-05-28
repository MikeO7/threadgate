package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

func TestThreadServiceBuildSnapshot(t *testing.T) {
	otctlRunner := FuncOtCtl(snapshotFixtureOtCtl(t))
	svc := NewThreadService(otctlRunner, CollectBestEffort)

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

func TestThreadServiceBuildSnapshotPartialFailure(t *testing.T) {
	otctlRunner := FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		key := otctl.Command{Args: args}.Key()
		if key == otctl.State.Key() {
			return "", context.Canceled
		}
		return "ok", nil
	})
	svc := NewThreadService(otctlRunner, CollectBestEffort)

	snap, err := svc.BuildSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected best-effort partial snapshot error")
	}
	if len(snap.Warnings) == 0 {
		t.Fatal("expected warnings on partial snapshot")
	}
}

func TestThreadServiceBuildSnapshotStrictMode(t *testing.T) {
	otctlRunner := FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == otctl.State.Args[0] {
			return "", context.Canceled
		}
		return "ok", nil
	})
	svc := NewThreadService(otctlRunner, CollectStrict)

	_, err := svc.BuildSnapshot(context.Background())
	if err == nil {
		t.Fatal("expected strict collection to fail on first error")
	}
}

func snapshotFixtureOtCtl(t *testing.T) func(context.Context, ...string) (string, error) {
	t.Helper()
	fixtures := map[string]string{
		otctl.State.Key():           "leader",
		otctl.NetworkName.Key():     testNetworkName,
		otctl.ExtAddr.Key():         "1122334455667788",
		otctl.PanID.Key():           "0x1234",
		otctl.Channel.Key():         "15",
		otctl.Rloc16.Key():          "0xc000",
		otctl.DatasetActive.Key():   activeDatasetHex,
		otctl.IPAddr.Key():          "fd00::1",
		otctl.Counters.Key():        "MacTxUnique=1",
		otctl.NeighborTable.Key():   readFixture(t, "neighbor_table.txt"),
		otctl.ChildTable.Key():      readFixture(t, "child_table.txt"),
		otctl.RouterTable.Key():     readFixture(t, "router_table.txt"),
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
