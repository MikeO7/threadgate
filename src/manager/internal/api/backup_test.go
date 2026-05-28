package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

func mockBackupOtCtl(calledActiveSet, calledActiveCommit, calledPendingSet, calledPendingCommit *bool) FuncOtCtl {
	handlers := map[string]func() (string, error){
		otctl.NetworkName.Key():    func() (string, error) { return testNetworkName, nil },
		otctl.PanID.Key():          func() (string, error) { return "0x1234", nil },
		otctl.Channel.Key():        func() (string, error) { return "15", nil },
		otctl.ExtAddr.Key():        func() (string, error) { return "1122334455667788", nil },
		otctl.DatasetActive.Key():  func() (string, error) { return activeDatasetHex, nil },
		otctl.DatasetPending.Key(): func() (string, error) { return pendingDatasetHex, nil },
		"dataset set active " + activeDatasetHex: func() (string, error) {
			*calledActiveSet = true
			return "", nil
		},
		otctl.DatasetCommitActive.Key(): func() (string, error) {
			*calledActiveCommit = true
			return "", nil
		},
		"dataset set pending " + pendingDatasetHex: func() (string, error) {
			*calledPendingSet = true
			return "", nil
		},
		otctl.DatasetCommitPending.Key(): func() (string, error) {
			*calledPendingCommit = true
			return "", nil
		},
	}
	return func(_ context.Context, args ...string) (string, error) {
		cmd := strings.Join(args, " ")
		if fn, ok := handlers[cmd]; ok {
			return fn()
		}
		return "", nil
	}
}

func TestHandleBackupExport(t *testing.T) {
	server := NewServerWithOtCtl(8081, mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup", nil)
	rr := httptest.NewRecorder()
	server.handleBackup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var backup ConfigBackup
	if err := json.Unmarshal(rr.Body.Bytes(), &backup); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if backup.Version != backupVersion {
		t.Errorf("expected version %d, got %d", backupVersion, backup.Version)
	}
	if backup.ActiveDataset != activeDatasetHex {
		t.Errorf("expected active dataset %q, got %q", activeDatasetHex, backup.ActiveDataset)
	}
	if backup.PendingDataset != pendingDatasetHex {
		t.Errorf("expected pending dataset %q, got %q", pendingDatasetHex, backup.PendingDataset)
	}
	if backup.NetworkName != testNetworkName {
		t.Errorf("expected network name %q, got %q", testNetworkName, backup.NetworkName)
	}
}

func TestHandleBackupImport(t *testing.T) {
	calledActiveSet := false
	calledActiveCommit := false
	calledPendingSet := false
	calledPendingCommit := false

	server := NewServerWithOtCtl(8081, mockBackupOtCtl(
		&calledActiveSet, &calledActiveCommit, &calledPendingSet, &calledPendingCommit,
	), false)

	payload := ConfigBackup{
		Version:        backupVersion,
		ActiveDataset:  activeDatasetHex,
		PendingDataset: pendingDatasetHex,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.handleBackup(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !calledActiveSet || !calledActiveCommit {
		t.Error("expected active dataset set and commit")
	}
	if !calledPendingSet || !calledPendingCommit {
		t.Error("expected pending dataset set and commit")
	}
}

func TestHandleBackupImportInvalid(t *testing.T) {
	server := NewServerWithOtCtl(8081, FuncOtCtl(func(context.Context, ...string) (string, error) {
		return "", nil
	}), false)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/import", strings.NewReader(`{"version":1}`))
	rr := httptest.NewRecorder()
	server.handleBackup(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing activeDataset, got %d", rr.Code)
	}
}

func TestHandleBackupSave(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort), false, dir, nil)

	reqSave := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/save", nil)
	rrSave := httptest.NewRecorder()
	server.handleBackup(rrSave, reqSave)
	if rrSave.Code != http.StatusOK {
		t.Fatalf("save expected 200, got %d: %s", rrSave.Code, rrSave.Body.String())
	}

	var saveResp map[string]string
	if err := json.Unmarshal(rrSave.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("invalid save response: %v", err)
	}
	filename := saveResp["filename"]
	if filename == "" {
		t.Fatal("expected filename in save response")
	}

	path := filepath.Join(dir, "backups", filename)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backup file not written: %v", err)
	}
}

func TestHandleBackupFilesList(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort), false, dir, nil)

	reqSave := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/save", nil)
	rrSave := httptest.NewRecorder()
	server.handleBackup(rrSave, reqSave)
	if rrSave.Code != http.StatusOK {
		t.Fatalf("save expected 200, got %d: %s", rrSave.Code, rrSave.Body.String())
	}

	var saveResp map[string]string
	if err := json.Unmarshal(rrSave.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("invalid save response: %v", err)
	}
	filename := saveResp["filename"]

	reqList := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/backup/files", nil)
	rrList := httptest.NewRecorder()
	server.handleBackup(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", rrList.Code)
	}

	var files []string
	if err := json.Unmarshal(rrList.Body.Bytes(), &files); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if len(files) != 1 || files[0] != filename {
		t.Errorf("expected [%q], got %v", filename, files)
	}
}

func TestHandleBackupFileRestore(t *testing.T) {
	dir := t.TempDir()
	server := NewServerWithThread(8081, NewThreadService(mockBackupOtCtl(new(bool), new(bool), new(bool), new(bool)), CollectBestEffort), false, dir, nil)

	reqSave := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/save", nil)
	rrSave := httptest.NewRecorder()
	server.handleBackup(rrSave, reqSave)
	if rrSave.Code != http.StatusOK {
		t.Fatalf("save expected 200, got %d: %s", rrSave.Code, rrSave.Body.String())
	}

	var saveResp map[string]string
	if err := json.Unmarshal(rrSave.Body.Bytes(), &saveResp); err != nil {
		t.Fatalf("invalid save response: %v", err)
	}
	filename := saveResp["filename"]

	calledActiveSet := false
	calledActiveCommit := false
	server.threads = NewThreadService(mockBackupOtCtl(
		&calledActiveSet, &calledActiveCommit, new(bool), new(bool),
	), CollectBestEffort)
	server.backup = NewBackupStore(server.threads, dir)

	reqRestore := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/backup/files/"+filename, nil)
	rrRestore := httptest.NewRecorder()
	server.handleBackup(rrRestore, reqRestore)
	if rrRestore.Code != http.StatusOK {
		t.Fatalf("restore expected 200, got %d: %s", rrRestore.Code, rrRestore.Body.String())
	}
	if !calledActiveSet || !calledActiveCommit {
		t.Error("expected active dataset restored from saved file")
	}
}
