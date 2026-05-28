package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

func TestHandleBackupFilesFiltersAndErrors(t *testing.T) {
	dir := t.TempDir()
	backupRoot := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupRoot, "keep.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupRoot, "skip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/files", nil)
	rr := httptest.NewRecorder()
	server.handleBackupFiles(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var files []string
	if err := json.Unmarshal(rr.Body.Bytes(), &files); err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "keep.json" {
		t.Fatalf("expected only keep.json, got %v", files)
	}

	if err := os.Chmod(backupRoot, 0); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(backupRoot, 0o750) }() //nolint:gosec // restore temp dir permissions for cleanup
	rrErr := httptest.NewRecorder()
	server.handleBackupFiles(rrErr, req)
	if rrErr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 listing failure, got %d", rrErr.Code)
	}
}

func TestHandleBackupFileGetNotFound(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/files/missing.json", nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileGet(rr, req, "missing.json")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleBackupFileGetInvalidName(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/files/", nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileGet(rr, req, "..")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleBackupFileRestoreImportError(t *testing.T) {
	dir := t.TempDir()
	backupRoot := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "valid.json"
	payload, _ := json.Marshal(ConfigBackup{ActiveDataset: activeDatasetHex})
	if err := os.WriteFile(filepath.Join(backupRoot, name), payload, 0o600); err != nil {
		t.Fatal(err)
	}

	server := NewServerWithThread(8081, NewThreadService(FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("import failed")
	}), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/files/"+name, nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileRestore(rr, req, name)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleBackupFileRestoreDefaultsVersion(t *testing.T) {
	dir := t.TempDir()
	backupRoot := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "legacy.json"
	if err := os.WriteFile(filepath.Join(backupRoot, name), []byte(`{"activeDataset":"`+activeDatasetHex+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	called := false
	server := NewServerWithThread(8081, NewThreadService(FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if strings.Join(args, " ") == "dataset set active "+activeDatasetHex {
			called = true
		}
		return "", nil
	}), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/files/"+name, nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileRestore(rr, req, name)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("expected legacy backup restore to import active dataset")
	}
}

func TestHandleBackupImportInvalidJSON(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/import", strings.NewReader("{"))
	rr := httptest.NewRecorder()
	server.handleBackupImport(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestExportBackupMetadataError(t *testing.T) {
	threads := NewThreadService(FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		cmd := strings.Join(args, " ")
		switch cmd {
		case otctl.DatasetActive.Key():
			return activeDatasetHex, nil
		case otctl.DatasetPending.Key():
			return "", nil
		case otctl.NetworkName.Key():
			return "", fmt.Errorf("network name unavailable")
		default:
			return "ok", nil
		}
	}), CollectBestEffort)
	store := NewBackupStore(threads, "")
	_, err := store.Export(context.Background())
	if err == nil {
		t.Fatal("expected metadata export error")
	}
}

func TestImportBackupPendingOnly(t *testing.T) {
	threads := NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort)
	store := NewBackupStore(threads, "")
	err := store.Import(context.Background(), ConfigBackup{
		ActiveDataset:  activeDatasetHex,
		PendingDataset: pendingDatasetHex,
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
}

func TestParseDatasetHexQuotedFallback(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/", strings.NewReader(`"`+activeDatasetHex+`"`))
	got, err := parseDatasetHex(req)
	if err != nil {
		t.Fatalf("expected quoted fallback parse, got error: %v", err)
	}
	if got != activeDatasetHex {
		t.Fatalf("unexpected fallback body: %q", got)
	}
}

func TestHandleHealthWriteError(t *testing.T) {
	_ = t
	tracker := runtime.NewTracker()
	tracker.UpdateRadioHealth("", "v1", "")
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", tracker)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/health", nil)
	server.handleHealth(&failResponseWriter{}, req)
}

func TestSetActiveDatasetEmptyBody(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/node/dataset/active", strings.NewReader(""))
	rr := httptest.NewRecorder()
	server.setActiveDataset(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleBackupFileRestoreNotFound(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/files/missing.json", nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileRestore(rr, req, "missing.json")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleBackupFileInvalidName(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/files/..", nil)
	req.URL.Path = "/api/backup/files/.."
	rr := httptest.NewRecorder()
	server.handleBackup(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetActiveDatasetWriteError(t *testing.T) {
	_ = t
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(_ context.Context, args ...string) (string, error) {
		if strings.Join(args, " ") == otctl.DatasetActive.Key() {
			return activeDatasetHex, nil
		}
		return "", fmt.Errorf("unexpected")
	}), false)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/node/dataset/active", nil)
	server.getActiveDataset(&failResponseWriter{}, req)
}

func TestSetPendingDatasetEmptyBody(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/node/dataset/pending", strings.NewReader(""))
	rr := httptest.NewRecorder()
	server.setPendingDataset(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleDashboardWriteError(t *testing.T) {
	_ = t
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), true, "", nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	server.handleDashboard(&failResponseWriter{}, req)
}

func TestHandleBackupImportMethodNotAllowed(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/import", nil)
	rr := httptest.NewRecorder()
	server.handleBackupImport(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestMarshalTopologyJSON(t *testing.T) {
	snap := topology.BuildFromTables("0xc000", "", "", "")
	data, err := MarshalTopologyJSON(snap)
	if err != nil {
		t.Fatalf("MarshalTopologyJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected topology JSON")
	}
}
