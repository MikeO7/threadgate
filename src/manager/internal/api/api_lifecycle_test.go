package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

func TestServerStartShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	server := NewServer(port, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", nil)
	errCh := make(chan error, 1)
	go func() { errCh <- server.Start() }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestShutdownWithoutStart(t *testing.T) {
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", nil)
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected nil shutdown, got %v", err)
	}
}

func TestHandleHealthWithoutReporter(t *testing.T) {
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", nil)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	server.handleHealth(rr, req)

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["status"] != "unknown" {
		t.Errorf("expected unknown status, got %q", resp["status"])
	}
}

func TestHandleDashboardNotFound(t *testing.T) {
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", nil)
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	server.handleDashboard(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestNewOtCtlModes(t *testing.T) {
	if _, ok := NewOtCtl(true).(*MockOtCtl); !ok {
		t.Fatal("expected mock otctl")
	}
	if _, ok := NewOtCtl(false).(ExecOtCtl); !ok {
		t.Fatal("expected exec otctl")
	}
}

func TestRunOtCtlWithContext(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ot-ctl")
	body := "#!/bin/sh\necho leader\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)

	out, err := runOtCtlWithContext(context.Background(), otctl.State.Args[0])
	if err != nil {
		t.Fatalf("runOtCtlWithContext failed: %v", err)
	}
	if out != "leader" {
		t.Errorf("expected leader, got %q", out)
	}
}

func TestExecOtCtlRun(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "ot-ctl")
	body := "#!/bin/sh\necho Done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)

	out, err := ExecOtCtl{}.Run(context.Background(), otctl.State.Args[0])
	if err != nil {
		t.Fatalf("ExecOtCtl.Run failed: %v", err)
	}
	if out != "" {
		t.Errorf("expected trimmed Done output, got %q", out)
	}
}

func TestHandleBackupFileGet(t *testing.T) {
	dir := t.TempDir()
	server := NewServer(8081, NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort), false, dir, nil)

	reqSave := httptest.NewRequest(http.MethodPost, "/api/backup/save", nil)
	rrSave := httptest.NewRecorder()
	server.handleBackup(rrSave, reqSave)
	if rrSave.Code != http.StatusOK {
		t.Fatalf("save expected 200, got %d", rrSave.Code)
	}

	var saveResp map[string]string
	if err := json.Unmarshal(rrSave.Body.Bytes(), &saveResp); err != nil {
		t.Fatal(err)
	}
	filename := saveResp["filename"]

	reqGet := httptest.NewRequest(http.MethodGet, "/api/backup/files/"+filename, nil)
	rrGet := httptest.NewRecorder()
	server.handleBackup(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d", rrGet.Code)
	}
}

func TestHandleBackupErrors(t *testing.T) {
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, "", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/backup/files", nil)
	rr := httptest.NewRecorder()
	server.handleBackup(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}

	dir := t.TempDir()
	serverWithDir := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	reqBad := httptest.NewRequest(http.MethodGet, "/api/backup/files/not-a-json-file", nil)
	rrBad := httptest.NewRecorder()
	serverWithDir.handleBackup(rrBad, reqBad)
	if rrBad.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing backup, got %d", rrBad.Code)
	}
}

func TestHandleNodeInfoError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", exec.ErrNotFound
	}), false)
	req := httptest.NewRequest(http.MethodGet, "/api/node", nil)
	rr := httptest.NewRecorder()
	server.handleNodeInfo(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleHealthWithReporter(t *testing.T) {
	tracker := runtime.NewTracker()
	tracker.UpdateRadioHealth("", "v1", "")
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", tracker)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rr := httptest.NewRecorder()
	server.handleHealth(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
