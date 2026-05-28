package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/thread"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

const (
	constActive     = "active"
	constPending    = "pending"
	constLeaderTest = "leader"
)

func mockNodeInfoOtCtl(_ context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no args")
	}
	switch args[0] {
	case otctl.State.Args[0]:
		return constLeaderTest, nil
	case otctl.Rloc16.Args[0]:
		return "0xc000", nil
	case otctl.ExtAddr.Args[0]:
		return "1122334455667788", nil
	case otctl.NetworkName.Args[0]:
		return testNetworkName, nil
	case otctl.PanID.Args[0]:
		return "0x1234", nil
	}
	return "", fmt.Errorf("unknown command: %v", args)
}

func TestHandleNodeInfo(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(mockNodeInfoOtCtl), false)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/node", nil)
	rr := httptest.NewRecorder()

	server.handleNodeInfo(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["State"] != constLeaderTest {
		t.Errorf("Expected State 'leader', got %v", resp["State"])
	}
	if resp["NetworkName"] != testNetworkName {
		t.Errorf("Expected NetworkName %q, got %v", testNetworkName, resp["NetworkName"])
	}
	if resp["PanId"] != "0x1234" {
		t.Errorf("Expected PanId '0x1234', got %v", resp["PanId"])
	}
}

func TestHandleDiagnostics(t *testing.T) {
	mockOtCtl := FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if len(args) == 0 {
			return "", nil
		}
		switch args[0] {
		case otctl.Counters.Args[0]:
			return "counter1\ncounter2", nil
		case otctl.IPAddr.Args[0]:
			return "fd00::1\nfd00::2", nil
		case "neighbor":
			return "neighbor1\nneighbor2", nil
		}
		return "", nil
	})

	server := NewServerWithOtCtl(8081, mockOtCtl, false)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/diagnostics", nil)
	rr := httptest.NewRecorder()

	server.handleDiagnostics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["IPAddresses"] == nil || resp["Counters"] == nil || resp["NeighborTable"] == nil {
		t.Errorf("Missing fields in diagnostics response: %v", resp)
	}
}

func TestHandleTopology(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), true)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/topology", nil)
	rr := httptest.NewRecorder()

	server.handleTopology(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var snap topology.Snapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("failed to decode topology: %v", err)
	}
	if snap.NetworkName != mockNetworkName {
		t.Errorf("expected mock network name, got %q", snap.NetworkName)
	}
	if len(snap.TrafficPaths) == 0 {
		t.Error("expected server-computed traffic paths in topology response")
	}
}

func TestHandleDashboardUsesConfiguredPort(t *testing.T) {
	server := NewServerWithThread(9090, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", nil)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/", nil)
	rr := httptest.NewRecorder()

	server.handleDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, ">9090<") {
		t.Error("expected dashboard to render configured port 9090")
	}
}

func TestMockMeshTables(t *testing.T) {
	svc := NewThreadService(NewMockOtCtl(), CollectBestEffort)
	snap, err := svc.BuildSnapshot(context.Background())
	if err != nil {
		t.Fatalf("BuildSnapshot failed: %v", err)
	}
	if len(snap.Neighbors) != thread.MockMeshNodeCount {
		t.Fatalf("expected %d mesh nodes, got %d", thread.MockMeshNodeCount, len(snap.Neighbors))
	}
	if len(snap.MeshLinks) < thread.MockDirectCount+thread.MockEndDeviceCount {
		t.Fatalf(
			"expected at least %d mesh links, got %d",
			thread.MockDirectCount+thread.MockEndDeviceCount,
			len(snap.MeshLinks),
		)
	}
}

func TestHandleHealth(t *testing.T) {
	tracker := runtime.NewTracker()
	tracker.UpdateRadioHealth("/dev/ttyTEST", "TestVersion/1.0", "")

	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", tracker)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var status runtime.Status
	if err := json.Unmarshal(rr.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}

	if status.ProbedVersion != "TestVersion/1.0" {
		t.Errorf("expected ProbedVersion 'TestVersion/1.0', got %q", status.ProbedVersion)
	}
	if status.RadioPath != "/dev/ttyTEST" {
		t.Errorf("expected RadioPath '/dev/ttyTEST', got %q", status.RadioPath)
	}
}
