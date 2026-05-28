package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupServiceExportImport(t *testing.T) {
	threads := NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort)
	store := NewBackupStore(threads, "")
	backup, err := store.Export(context.Background())
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if backup.ActiveDataset != activeDatasetHex {
		t.Errorf("unexpected active dataset: %q", backup.ActiveDataset)
	}

	if err := store.Import(context.Background(), backup); err != nil {
		t.Fatalf("Import failed: %v", err)
	}
}

func TestValidateConfigBackup(t *testing.T) {
	if err := ValidateConfigBackup(ConfigBackup{ActiveDataset: activeDatasetHex}); err != nil {
		t.Fatalf("expected valid backup: %v", err)
	}
	if err := ValidateConfigBackup(ConfigBackup{}); err == nil {
		t.Fatal("expected missing active dataset error")
	}
	if err := ValidateConfigBackup(ConfigBackup{ActiveDataset: "zz"}); err == nil {
		t.Fatal("expected invalid hex error")
	}
	if err := ValidateConfigBackup(ConfigBackup{ActiveDataset: activeDatasetHex, PendingDataset: "bad"}); err == nil {
		t.Fatal("expected invalid pending dataset error")
	}
}

func TestParseConfigBackup(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"version":2}`))
	_, err := ParseConfigBackupRequest(req)
	if err == nil {
		t.Fatal("expected unsupported version error")
	}

	reqDefault := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", strings.NewReader(`{"activeDataset":"`+activeDatasetHex+`"}`))
	backup, err := ParseConfigBackupRequest(reqDefault)
	if err != nil {
		t.Fatalf("ParseConfigBackupRequest failed: %v", err)
	}
	if backup.Version != backupVersion {
		t.Fatalf("expected version defaulting, got %d", backup.Version)
	}
}

func TestValidateBackupFilename(t *testing.T) {
	if err := validateBackupFilename("backup.json"); err != nil {
		t.Fatalf("expected valid filename: %v", err)
	}
	if err := validateBackupFilename(""); err == nil {
		t.Fatal("expected invalid filename")
	}
	if err := validateBackupFilename(".."); err == nil {
		t.Fatal("expected invalid filename")
	}
}

func TestHandleBackupExportMethodNotAllowed(t *testing.T) {
	server := NewServerWithOtCtl(8081, NewMockOtCtl(), false)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup", nil)
	rr := httptest.NewRecorder()
	server.handleBackupExport(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.Code)
	}
}

func TestHandleBackupFilesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/files", nil)
	rr := httptest.NewRecorder()
	server.handleBackupFiles(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestHandleBackupFileRestoreInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	backupRoot := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	name := "broken.json"
	if err := os.WriteFile(filepath.Join(backupRoot, name), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := NewServerWithThread(8081, NewThreadService(NewMockOtCtl(), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/files/"+name, nil)
	rr := httptest.NewRecorder()
	server.handleBackupFileRestore(rr, req, name)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestHandleBackupImportFailure(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("ot-ctl down")
	}), false)

	payload, _ := json.Marshal(ConfigBackup{Version: backupVersion, ActiveDataset: activeDatasetHex})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/import", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	server.handleBackupImport(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestRunMockDatasetCommand(t *testing.T) {
	otctl := NewMockOtCtl()
	if _, err := otctl.Run(context.Background(), "dataset", "active", "-x"); err != nil {
		t.Fatalf("dataset active failed: %v", err)
	}
	if _, err := otctl.Run(context.Background(), "dataset", "pending", "-x"); err != nil {
		t.Fatalf("dataset pending failed: %v", err)
	}
}

func TestHandleBackupSaveExportError(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", fmt.Errorf("export failed")
	}), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/save", nil)
	rr := httptest.NewRecorder()
	server.handleBackupSave(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestHandleBackupSaveWriteFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "backups"), []byte("not-a-dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := NewServerWithThread(8081, NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort), false, dir, nil)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/save", nil)
	rr := httptest.NewRecorder()
	server.handleBackupSave(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
