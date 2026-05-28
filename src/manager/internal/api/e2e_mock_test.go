package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/api/topology"
)

func TestE2EMockMode(t *testing.T) {
	mock := NewMockOtCtl()
	server := NewServerWithOtCtl(0, mock, true)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)

	baseURL := ts.URL

	t.Run("NodeInfo", func(t *testing.T) {
		testNodeInfo(t, baseURL)
	})

	t.Run("ActiveDatasetGetDefault", func(t *testing.T) {
		testActiveDatasetGetDefault(t, baseURL)
	})

	t.Run("ActiveDatasetPutAndGet", func(t *testing.T) {
		testActiveDatasetPutAndGet(t, baseURL)
	})

	t.Run("ActiveDatasetJSONPutAndGet", func(t *testing.T) {
		testActiveDatasetJSONPutAndGet(t, baseURL)
	})

	t.Run("Diagnostics", func(t *testing.T) {
		testDiagnostics(t, baseURL)
	})

	t.Run("Dashboard", func(t *testing.T) {
		testDashboard(t, baseURL)
	})

	t.Run("Topology", func(t *testing.T) {
		testTopology(t, baseURL)
	})
}

func testNodeInfo(t *testing.T, baseURL string) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/node", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed E2E GET /api/node: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var nodeInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&nodeInfo); err != nil {
		t.Fatalf("Failed to decode node info json: %v", err)
	}

	if nodeInfo["NetworkName"] != mockNetworkName {
		t.Errorf("Expected mock network name %q, got %v", mockNetworkName, nodeInfo["NetworkName"])
	}
	if nodeInfo["State"] != "leader" {
		t.Errorf("Expected mock state 'leader', got %v", nodeInfo["State"])
	}
}

func testActiveDatasetGetDefault(t *testing.T, baseURL string) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/node/dataset/active", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed E2E GET /node/dataset/active: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != activeDatasetHex {
		t.Errorf("Expected default active dataset hex, got %q", string(body))
	}
}

func testActiveDatasetPutAndGet(t *testing.T, baseURL string) {
	ctx := context.Background()
	newHex := "0e080000000000018888"
	reqPut, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/node/dataset/active", strings.NewReader(newHex))
	if err != nil {
		t.Fatalf("Failed to create PUT request: %v", err)
	}
	respPut, err := http.DefaultClient.Do(reqPut)
	if err != nil {
		t.Fatalf("Failed E2E PUT /node/dataset/active: %v", err)
	}
	_ = respPut.Body.Close()

	if respPut.StatusCode != http.StatusOK {
		t.Errorf("Expected PUT status 200, got %d", respPut.StatusCode)
	}

	reqGet, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/node/dataset/active", nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}
	resp, err := http.DefaultClient.Do(reqGet)
	if err != nil {
		t.Fatalf("Failed E2E GET /node/dataset/active after update: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != newHex {
		t.Errorf("Expected updated active dataset hex %q, got %q", newHex, string(body))
	}
}

func testActiveDatasetJSONPutAndGet(t *testing.T, baseURL string) {
	ctx := context.Background()
	jsonPayload := `{"ActiveDatasetTlvs": "0e080000000000017777"}`
	reqPutJSON, err := http.NewRequestWithContext(ctx, http.MethodPut, baseURL+"/api/node/dataset/active", bytes.NewBufferString(jsonPayload))
	if err != nil {
		t.Fatalf("Failed to create JSON PUT request: %v", err)
	}
	reqPutJSON.Header.Set("Content-Type", "application/json")
	respPutJSON, err := http.DefaultClient.Do(reqPutJSON)
	if err != nil {
		t.Fatalf("Failed E2E JSON PUT /api/node/dataset/active: %v", err)
	}
	_ = respPutJSON.Body.Close()

	if respPutJSON.StatusCode != http.StatusOK {
		t.Errorf("Expected JSON PUT status 200, got %d", respPutJSON.StatusCode)
	}

	reqGet, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/node/dataset/active", nil)
	if err != nil {
		t.Fatalf("Failed to create GET request: %v", err)
	}
	resp, err := http.DefaultClient.Do(reqGet)
	if err != nil {
		t.Fatalf("Failed E2E GET after JSON update: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "0e080000000000017777" {
		t.Errorf("Expected JSON updated active dataset hex, got %q", string(body))
	}
}

func testDiagnostics(t *testing.T, baseURL string) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/diagnostics", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed E2E GET /api/diagnostics: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var diagInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&diagInfo); err != nil {
		t.Fatalf("Failed to decode diagnostics JSON: %v", err)
	}
	if diagInfo["IPAddresses"] == nil {
		t.Errorf("Expected IPAddresses in diagnostics, got nil")
	}
}

func testDashboard(t *testing.T, baseURL string) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed E2E GET dashboard: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected dashboard status 200, got %d", resp.StatusCode)
	}
	bodyDash, _ := io.ReadAll(resp.Body)
	body := string(bodyDash)
	if !strings.Contains(body, "ThreadGate-Mock") {
		t.Errorf("Expected dashboard to render simulated network name 'ThreadGate-Mock'")
	}
	if !strings.Contains(body, "trafficPaths") {
		t.Error("expected embedded topology JSON to include server traffic paths")
	}
}

func testTopology(t *testing.T, baseURL string) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/topology", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed E2E GET /api/topology: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected topology status 200, got %d", resp.StatusCode)
	}

	var snap topology.Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		t.Fatalf("Failed to decode topology JSON: %v", err)
	}
	if len(snap.TrafficPaths) == 0 {
		t.Error("Expected traffic paths in topology API response")
	}
}
