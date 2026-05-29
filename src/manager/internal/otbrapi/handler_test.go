package otbrapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

func TestHandleCoprocessorVersion(t *testing.T) {
	tracker := runtime.NewTracker()
	tracker.UpdateRadioHealth("/dev/ttyUSB0", "RCP/2.0.0-test", "")
	h := &Handler{
		Ops:    &ClientAdapter{Client: thread.NewClient(thread.NewMock(), thread.PolicyBestEffort)},
		Status: tracker,
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/node/coprocessor/version", nil)
	rr := httptest.NewRecorder()
	h.HandleCoprocessorVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var got string
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != "RCP/2.0.0-test" {
		t.Fatalf("got %q want RCP/2.0.0-test", got)
	}
}

func TestHandleNodeState(t *testing.T) {
	mock := thread.NewMock()
	h := &Handler{Ops: &ClientAdapter{Client: thread.NewClient(mock, thread.PolicyBestEffort)}}

	for _, state := range []string{nodeStateEnable, nodeStateDisable} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/node/state", strings.NewReader(`"`+state+`"`))
		rr := httptest.NewRecorder()
		h.HandleNodeState(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("state %q: status %d body %s", state, rr.Code, rr.Body.String())
		}
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/node/state", strings.NewReader(`"invalid"`))
	rr := httptest.NewRecorder()
	h.HandleNodeState(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid state, got %d", rr.Code)
	}
}

func TestHandleActiveDataset(t *testing.T) {
	mock := thread.NewMock()
	h := &Handler{Ops: &ClientAdapter{Client: thread.NewClient(mock, thread.PolicyBestEffort)}}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/node/dataset/active", nil)
	rr := httptest.NewRecorder()
	h.HandleActiveDataset(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), thread.MockActiveDataset[:8]) {
		t.Fatalf("unexpected active dataset body: %s", rr.Body.String())
	}
}

func TestNormalizeHexID(t *testing.T) {
	if got := NormalizeHexID("0x11:22:33"); got != "112233" {
		t.Fatalf("got %q", got)
	}
}
