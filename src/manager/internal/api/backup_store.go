package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

const backupVersion = 1

// ConfigBackup bundles Thread network credentials for export and restore.
type ConfigBackup struct {
	Version        int    `json:"version"`
	ExportedAt     string `json:"exportedAt"`
	NetworkName    string `json:"networkName,omitempty"`
	PanID          string `json:"panId,omitempty"`
	Channel        string `json:"channel,omitempty"`
	ExtAddress     string `json:"extAddress,omitempty"`
	ActiveDataset  string `json:"activeDataset"`
	PendingDataset string `json:"pendingDataset,omitempty"`
}

// BackupStore handles Thread credential export/import and on-disk backup files.
type BackupStore struct {
	threads  *ThreadService
	stateDir string
}

// NewBackupStore wires backup operations to a ThreadService and optional state directory.
func NewBackupStore(threads *ThreadService, stateDir string) *BackupStore {
	return &BackupStore{threads: threads, stateDir: stateDir}
}

// Export collects live network credentials from ot-ctl.
func (b *BackupStore) Export(ctx context.Context) (ConfigBackup, error) {
	var backup ConfigBackup
	var errs []error

	backup.Version = backupVersion
	backup.ExportedAt = time.Now().UTC().Format(time.RFC3339)

	if v, err := b.threads.run(ctx, otctl.NetworkName.Args...); err != nil {
		errs = append(errs, err)
	} else {
		backup.NetworkName = v
	}
	if v, err := b.threads.run(ctx, otctl.PanID.Args...); err != nil {
		errs = append(errs, err)
	} else {
		backup.PanID = v
	}
	if v, err := b.threads.run(ctx, otctl.Channel.Args...); err != nil {
		errs = append(errs, err)
	} else {
		backup.Channel = v
	}
	if v, err := b.threads.run(ctx, otctl.ExtAddr.Args...); err != nil {
		errs = append(errs, err)
	} else {
		backup.ExtAddress = v
	}

	active, err := b.threads.GetActiveDataset(ctx)
	if err != nil {
		return backup, fmt.Errorf("active dataset: %w", err)
	}
	backup.ActiveDataset = strings.TrimSpace(active)

	pending, err := b.threads.GetPendingDataset(ctx)
	if err == nil {
		backup.PendingDataset = strings.TrimSpace(pending)
	}

	if len(errs) > 0 {
		return backup, fmt.Errorf("backup metadata: %w", errs[0])
	}
	return backup, nil
}

// Import restores network credentials from a backup bundle.
func (b *BackupStore) Import(ctx context.Context, backup ConfigBackup) error {
	active := strings.TrimSpace(backup.ActiveDataset)
	if err := b.threads.SetActiveDataset(ctx, active); err != nil {
		return fmt.Errorf("set active dataset: %w", err)
	}

	pending := strings.TrimSpace(backup.PendingDataset)
	if pending != "" {
		if err := b.threads.SetPendingDataset(ctx, pending); err != nil {
			return fmt.Errorf("set pending dataset: %w", err)
		}
	}
	return nil
}

// ParseRequestBody decodes a JSON backup from an HTTP request body.
func ParseConfigBackupRequest(r *http.Request) (ConfigBackup, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ConfigBackup{}, fmt.Errorf("read body: %w", err)
	}
	defer func() {
		_ = r.Body.Close()
	}()

	var backup ConfigBackup
	if err := json.Unmarshal(body, &backup); err != nil {
		return ConfigBackup{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if backup.Version == 0 {
		backup.Version = backupVersion
	}
	if backup.Version != backupVersion {
		return ConfigBackup{}, fmt.Errorf("unsupported backup version %d", backup.Version)
	}
	return backup, nil
}

// Validate checks required backup fields.
func ValidateConfigBackup(backup ConfigBackup) error {
	active := strings.TrimSpace(backup.ActiveDataset)
	if active == "" {
		return fmt.Errorf("activeDataset is required")
	}
	if !isValidHex(active) {
		return fmt.Errorf("activeDataset must be a hex-encoded TLV string")
	}
	pending := strings.TrimSpace(backup.PendingDataset)
	if pending != "" && !isValidHex(pending) {
		return fmt.Errorf("pendingDataset must be a hex-encoded TLV string")
	}
	return nil
}

func (b *BackupStore) backupDir() string {
	return filepath.Join(b.stateDir, "backups")
}

func validateBackupFilename(name string) error {
	name = filepath.Base(name)
	if name == "" || name == "." || strings.Contains(name, "..") {
		return fmt.Errorf("invalid backup filename")
	}
	return nil
}

// ListFiles returns JSON backup filenames stored on disk.
func (b *BackupStore) ListFiles() ([]string, error) {
	dir := b.backupDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		files = append(files, e.Name())
	}
	return files, nil
}

// ReadFile loads a stored backup by filename.
func (b *BackupStore) ReadFile(name string) ([]byte, error) {
	if err := validateBackupFilename(name); err != nil {
		return nil, err
	}
	path := filepath.Join(b.backupDir(), name)
	//nolint:gosec // G304: filename validated above to exclude path traversal
	return os.ReadFile(path)
}

// Save exports live credentials and writes them to the backups directory.
func (b *BackupStore) Save(ctx context.Context) (filename, path string, err error) {
	backup, err := b.Export(ctx)
	if err != nil {
		return "", "", err
	}

	dir := b.backupDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", "", err
	}

	filename = fmt.Sprintf("threadgate-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	path = filepath.Join(dir, filename)

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", "", err
	}
	return filename, path, nil
}
