package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	case otctlCmdState:
		return constLeaderTest, nil
	case otctlCmdRloc16:
		return "0xc000", nil
	case otctlCmdExtAddr:
		return "1122334455667788", nil
	case otctlCmdNetworkName:
		return testNetworkName, nil
	case otctlCmdPanID:
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
		case otctlCmdCounters:
			return "counter1\ncounter2", nil
		case otctlCmdIPAddr:
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

	var snap Snapshot
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
	server := NewServer(9090, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "")
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
	otctl := NewMockOtCtl()
	ctx := context.Background()

	neighborOutput, err := otctl.Run(ctx, "neighbor", "table")
	if err != nil {
		t.Fatalf("neighbor table failed: %v", err)
	}
	neighbors := parseNeighborTable(neighborOutput)
	if len(neighbors) != mockDirectCount {
		t.Fatalf("expected %d direct mock neighbors, got %d", mockDirectCount, len(neighbors))
	}

	routerOutput, err := otctl.Run(ctx, "router", "table")
	if err != nil {
		t.Fatalf("router table failed: %v", err)
	}
	routers := parseRouterTable(routerOutput)
	if len(routers) != mockNodeCount {
		t.Fatalf("expected %d mock routers, got %d", mockNodeCount, len(routers))
	}

	childOutput, err := otctl.Run(ctx, "child", "table")
	if err != nil {
		t.Fatalf("child table failed: %v", err)
	}
	children := parseChildTable(childOutput)
	if len(children) != 4 {
		t.Fatalf("expected 4 mock children, got %d", len(children))
	}

	meshNodes := mergeMeshNodes(neighbors, children, routers)
	if len(meshNodes) != mockNodeCount {
		t.Fatalf("expected %d mesh nodes, got %d", mockNodeCount, len(meshNodes))
	}

	links := buildMeshLinks("0xc000", neighbors, children, routers)
	if len(links) < mockDirectCount {
		t.Fatalf("expected at least %d mesh links, got %d", mockDirectCount, len(links))
	}
}
