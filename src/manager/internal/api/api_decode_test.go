package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

func TestHandleActiveDatasetDecode(t *testing.T) {
	mockOtCtl := thread.NewMock()
	server := NewServerWithOtCtl(8081, mockOtCtl, true)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/node/dataset/active/decode", nil)
	rr := httptest.NewRecorder()

	server.handleActiveDatasetDecode(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	var decoded thread.DecodedDataset
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if decoded.NetworkName != "Thread-Test" || decoded.Channel != 15 || decoded.PanID != "0x1234" {
		t.Errorf("Unexpected decoded mock dataset: %+v", decoded)
	}
}

func TestHandleDatasetDecodePost(t *testing.T) {
	server := NewServerWithOtCtl(8081, thread.NewMock(), true)

	// Hex for PAN ID 0x1234, Network Name 'Thread-Test'
	hexDataset := "01021234030b5468726561642d54657374"
	req := httptest.NewRequestWithContext(context.Background(), "POST", "/api/node/dataset/decode", strings.NewReader(hexDataset))
	rr := httptest.NewRecorder()

	server.handleDatasetDecode(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	var decoded thread.DecodedDataset
	if err := json.Unmarshal(rr.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if decoded.NetworkName != "Thread-Test" || decoded.PanID != "0x1234" {
		t.Errorf("Decoded unexpected fields: %+v", decoded)
	}
}

func TestHandleHealthcheck(t *testing.T) {
	server := NewServerWithOtCtl(8081, thread.NewMock(), true)

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/diagnostics/healthcheck", nil)
	rr := httptest.NewRecorder()

	server.handleHealthcheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	var summary thread.HealthSummary
	if err := json.Unmarshal(rr.Body.Bytes(), &summary); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !summary.Healthy {
		t.Errorf("Expected summary healthy to be true, got %v", summary.Healthy)
	}
}
