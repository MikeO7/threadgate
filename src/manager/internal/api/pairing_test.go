package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/pairing"
)

func defaultPairInitBody() map[string]string {
	return map[string]string{
		"app_name":   "Home Assistant Mock",
		"app_url":    "http://127.0.0.1:8123",
		"hass_token": "mock-long-lived-token",
	}
}

func pairingTestServer(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "threadgate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	cfg := &config.Config{
		StateDir: tempDir,
		Runtime:  config.RuntimeModeHardware,
	}
	server := NewServer(testEnv(nil, false), 0, tempDir)
	server.env.Config = cfg
	return server, server.Handler(), tempDir
}

func pairServeJSON(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody []byte
	if body != nil {
		var err error
		reqBody, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewBuffer(reqBody))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func pairInitiate(t *testing.T, handler http.Handler) pairing.Request {
	t.Helper()
	w := pairServeJSON(t, handler, http.MethodPost, "/api/pair/initiate", defaultPairInitBody())
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from initiate, got %d", w.Code)
	}
	var initResp pairing.Request
	if err := json.NewDecoder(w.Body).Decode(&initResp); err != nil {
		t.Fatalf("failed to decode initiate response: %v", err)
	}
	if initResp.ID == "" || initResp.Status != pairing.StatusPending {
		t.Errorf("unexpected initiate response fields: %+v", initResp)
	}
	return initResp
}

func pairAssertActive(t *testing.T, handler http.Handler, wantID string) {
	t.Helper()
	w := pairServeJSON(t, handler, http.MethodGet, "/api/pair/active", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from active, got %d", w.Code)
	}
	var activeList []*pairing.Request
	if err := json.NewDecoder(w.Body).Decode(&activeList); err != nil {
		t.Fatalf("failed to decode active list: %v", err)
	}
	if len(activeList) != 1 || activeList[0].ID != wantID {
		t.Errorf("unexpected active requests list size/content: %d", len(activeList))
	}
}

func pairAssertStatus(t *testing.T, handler http.Handler, id, want string) {
	t.Helper()
	w := pairServeJSON(t, handler, http.MethodGet, "/api/pair/status?id="+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from status, got %d", w.Code)
	}
	var statusResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&statusResp); err != nil {
		t.Fatalf("failed to decode status response: %v", err)
	}
	if statusResp[jsonKeyStatus] != want {
		t.Errorf("expected %q, got %q", want, statusResp[jsonKeyStatus])
	}
}

func pairApprove(t *testing.T, handler http.Handler, id string) {
	t.Helper()
	w := pairServeJSON(t, handler, http.MethodPost, "/api/pair/approve", map[string]string{"pairing_id": id})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from approve, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func pairAssertConfigSaved(t *testing.T, stateDir string) {
	t.Helper()
	cfgPath := filepath.Join(stateDir, "hass_config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Errorf("expected config file to be created at %s", cfgPath)
	}
}

func TestPairingFlow(t *testing.T) {
	server, handler, tempDir := pairingTestServer(t)

	initResp := pairInitiate(t, handler)
	pairAssertActive(t, handler, initResp.ID)
	pairAssertStatus(t, handler, initResp.ID, pairing.StatusPending)

	pairApprove(t, handler, initResp.ID)
	pairAssertConfigSaved(t, tempDir)
	pairAssertStatus(t, handler, initResp.ID, pairing.StatusApproved)

	if !server.hassClient.Enabled() {
		t.Error("expected HASS client to be enabled after dynamic reload")
	}
}

func TestPairingDenyFlow(t *testing.T) {
	server := NewServer(testEnv(nil, false), 0, "")
	handler := server.Handler()

	initResp := pairInitiate(t, handler)

	w := pairServeJSON(t, handler, http.MethodPost, "/api/pair/deny", map[string]string{"pairing_id": initResp.ID})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from deny, got %d", w.Code)
	}

	pairAssertStatus(t, handler, initResp.ID, pairing.StatusDenied)
}
