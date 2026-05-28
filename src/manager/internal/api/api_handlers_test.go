package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

func TestHandleNodeInfoWriteError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(mockNodeInfoOtCtl), false)
	req := httptest.NewRequest(http.MethodGet, "/api/node", nil)
	server.handleNodeInfo(&failResponseWriter{}, req)
}

type failResponseWriter struct {
	header http.Header
}

func (f *failResponseWriter) Header() http.Header {
	if f.header == nil {
		f.header = make(http.Header)
	}
	return f.header
}

func (f *failResponseWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write failed") }
func (f *failResponseWriter) WriteHeader(int)           {}

func TestGetActiveDatasetError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("dataset unavailable")
	}), false)
	req := httptest.NewRequest(http.MethodGet, "/node/dataset/active", nil)
	rr := httptest.NewRecorder()
	server.getActiveDataset(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestSetActiveDatasetServiceError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		return "", fmt.Errorf("commit failed")
	}), false)
	req := httptest.NewRequest(http.MethodPut, "/node/dataset/active", strings.NewReader(activeDatasetHex))
	rr := httptest.NewRecorder()
	server.setActiveDataset(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestGetPendingDatasetError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("pending unavailable")
	}), false)
	req := httptest.NewRequest(http.MethodGet, "/node/dataset/pending", nil)
	rr := httptest.NewRecorder()
	server.getPendingDataset(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleBackupSaveNoStateDir(t *testing.T) {
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, "", nil)
	req := httptest.NewRequest(http.MethodPost, "/api/backup/save", nil)
	rr := httptest.NewRecorder()
	server.handleBackupSave(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleBackupFileMethodNotAllowed(t *testing.T) {
	dir := t.TempDir()
	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequest(http.MethodDelete, "/api/backup/files/test.json", nil)
	rr := httptest.NewRecorder()
	server.handleBackupFile(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleBackupFileRestoreValidationFailure(t *testing.T) {
	dir := t.TempDir()
	backupDirPath := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDirPath, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "invalid-backup.json"
	if err := os.WriteFile(filepath.Join(backupDirPath, name), []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	server := NewServer(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/backup/files/"+name, nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileRestore(rr, req, name)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandlePendingDatasetMethodNotAllowed(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequest(http.MethodPost, "/node/dataset/pending", nil)
	rr := httptest.NewRecorder()
	server.handlePendingDataset(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestSetPendingDatasetServiceError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("pending commit failed")
	}), false)
	req := httptest.NewRequest(http.MethodPut, "/node/dataset/pending", strings.NewReader(pendingDatasetHex))
	rr := httptest.NewRecorder()
	server.setPendingDataset(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleDiagnosticsError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("diag failed")
	}), false)
	req := httptest.NewRequest(http.MethodGet, "/api/diagnostics", nil)
	rr := httptest.NewRecorder()
	server.handleDiagnostics(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleTopologyPartialSnapshot(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == otctl.State.Args[0] {
			return "", fmt.Errorf("state unavailable")
		}
		return "ok", nil
	}), false)
	req := httptest.NewRequest(http.MethodGet, "/api/topology", nil)
	rr := httptest.NewRecorder()
	server.handleTopology(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected partial topology 200, got %d", rr.Code)
	}
}

func TestHandleBackupExportError(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("export failed")
	}), false)
	req := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
	rr := httptest.NewRecorder()
	server.handleBackupExport(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleBackupNotFoundRoute(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/unknown", nil)
	rr := httptest.NewRecorder()
	server.handleBackup(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestThreadServiceNodeInfoPartialError(t *testing.T) {
	svc := NewThreadService(FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == otctl.State.Args[0] {
			return "", fmt.Errorf("state failed")
		}
		return "ok", nil
	}), CollectBestEffort)
	_, err := svc.NodeInfo(context.Background())
	if err == nil {
		t.Fatal("expected node info error")
	}
}

func TestThreadServiceDiagnosticsPartialError(t *testing.T) {
	svc := NewThreadService(FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if len(args) > 0 && args[0] == otctl.Counters.Args[0] {
			return "", fmt.Errorf("counters failed")
		}
		return "line", nil
	}), CollectBestEffort)
	_, err := svc.Diagnostics(context.Background())
	if err == nil {
		t.Fatal("expected diagnostics error")
	}
}

func TestSplitLines(t *testing.T) {
	if splitLines("") != nil {
		t.Fatal("expected nil for empty string")
	}
	lines := splitLines("a\nb")
	if len(lines) != 2 || lines[0] != "a" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestMockDatasetPendingCommands(t *testing.T) {
	otctl := NewMockOtCtl()
	ctx := context.Background()
	if _, err := otctl.Run(ctx, "dataset", "set", "pending", pendingDatasetHex); err != nil {
		t.Fatalf("set pending failed: %v", err)
	}
	if _, err := otctl.Run(ctx, "dataset", "commit", "pending"); err != nil {
		t.Fatalf("commit pending failed: %v", err)
	}
}

func TestRunOtCtlWithContextFailure(t *testing.T) {
	_, err := runOtCtlWithContext(context.Background(), "definitely-not-a-command")
	if err == nil {
		t.Fatal("expected ot-ctl failure")
	}
}
