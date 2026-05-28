package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleActiveDatasetGet(t *testing.T) {
	mockOtCtl := FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if strings.Join(args, " ") == otctlCmdDatasetActiveX {
			return activeDatasetHex, nil
		}
		return "", fmt.Errorf("unexpected command")
	})

	server := NewServerWithOtCtl(8081, mockOtCtl, false)
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/node/dataset/active", nil)
	rr := httptest.NewRecorder()

	server.handleActiveDataset(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if body != activeDatasetHex {
		t.Errorf("Expected active dataset hex, got %q", body)
	}
}

func mockActiveOtCtl(calledSet *bool, calledCommit *bool) FuncOtCtl {
	return func(_ context.Context, args ...string) (string, error) {
		cmdStr := strings.Join(args, " ")
		if cmdStr == "dataset set active 0e080000000000010000" {
			*calledSet = true
			return "", nil
		}
		if cmdStr == otctlCmdDatasetCommitActive {
			*calledCommit = true
			return "", nil
		}
		return "", fmt.Errorf("unexpected command: %s", cmdStr)
	}
}

func TestHandleActiveDatasetPutRawHex(t *testing.T) {
	calledSet := false
	calledCommit := false

	server := NewServerWithOtCtl(8081, mockActiveOtCtl(&calledSet, &calledCommit), false)
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/node/dataset/active", strings.NewReader(activeDatasetHex))
	rr := httptest.NewRecorder()

	server.handleActiveDataset(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	if !calledSet || !calledCommit {
		t.Errorf("Expected set and commit to be called")
	}
}

func TestHandleActiveDatasetPutJSON(t *testing.T) {
	calledSet := false
	calledCommit := false

	server := NewServerWithOtCtl(8081, mockActiveOtCtl(&calledSet, &calledCommit), false)
	jsonPayload := `{"ActiveDatasetTlvs": "` + activeDatasetHex + `"}`
	req := httptest.NewRequestWithContext(context.Background(), "PUT", "/node/dataset/active", bytes.NewBufferString(jsonPayload))
	rr := httptest.NewRecorder()

	server.handleActiveDataset(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	if !calledSet || !calledCommit {
		t.Errorf("Expected set and commit to be called")
	}
}

func TestHandleActiveDatasetPutInvalid(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", nil
	}), false)

	req1 := httptest.NewRequestWithContext(context.Background(), "PUT", "/node/dataset/active", strings.NewReader("not-hex-at-all"))
	rr1 := httptest.NewRecorder()
	server.handleActiveDataset(rr1, req1)
	if rr1.Code != http.StatusBadRequest {
		t.Errorf("Expected bad request for invalid hex, got %d", rr1.Code)
	}

	req2 := httptest.NewRequestWithContext(context.Background(), "POST", "/node/dataset/active", nil)
	rr2 := httptest.NewRecorder()
	server.handleActiveDataset(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected method not allowed, got %d", rr2.Code)
	}
}

func mockPendingOtCtl(calledSet *bool, calledCommit *bool) FuncOtCtl {
	return func(_ context.Context, args ...string) (string, error) {
		cmdStr := strings.Join(args, " ")
		if cmdStr == otctlCmdDatasetPendingX {
			return pendingDatasetHex, nil
		}
		if cmdStr == "dataset set pending 0e080000000000019999" {
			*calledSet = true
			return "", nil
		}
		if cmdStr == otctlCmdDatasetCommitPending {
			*calledCommit = true
			return "", nil
		}
		return "", fmt.Errorf("unexpected command: %s", cmdStr)
	}
}

func TestHandlePendingDataset(t *testing.T) {
	calledSet := false
	calledCommit := false

	server := NewServerWithOtCtl(8081, mockPendingOtCtl(&calledSet, &calledCommit), false)

	reqGet := httptest.NewRequestWithContext(context.Background(), "GET", "/node/dataset/pending", nil)
	rrGet := httptest.NewRecorder()
	server.handlePendingDataset(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Errorf("Expected GET status 200, got %d", rrGet.Code)
	}
	if rrGet.Body.String() != pendingDatasetHex {
		t.Errorf("Expected pending dataset hex, got %q", rrGet.Body.String())
	}

	reqPut := httptest.NewRequestWithContext(context.Background(), "PUT", "/node/dataset/pending", strings.NewReader(pendingDatasetHex))
	rrPut := httptest.NewRecorder()
	server.handlePendingDataset(rrPut, reqPut)
	if rrPut.Code != http.StatusOK {
		t.Errorf("Expected PUT status 200, got %d. Body: %s", rrPut.Code, rrPut.Body.String())
	}
	if !calledSet || !calledCommit {
		t.Errorf("Expected pending set and commit to be called")
	}
}
