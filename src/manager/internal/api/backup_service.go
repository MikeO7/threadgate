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

// ExportBackup collects live network credentials from ot-ctl.
func (s *ThreadService) ExportBackup(ctx context.Context) (ConfigBackup, error) {
	var backup ConfigBackup
	var errs []error

	backup.Version = backupVersion
	backup.ExportedAt = time.Now().UTC().Format(time.RFC3339)

	if v, err := s.run(ctx, otctlCmdNetworkName); err != nil {
		errs = append(errs, err)
	} else {
		backup.NetworkName = v
	}
	if v, err := s.run(ctx, otctlCmdPanID); err != nil {
		errs = append(errs, err)
	} else {
		backup.PanID = v
	}
	if v, err := s.run(ctx, "channel"); err != nil {
		errs = append(errs, err)
	} else {
		backup.Channel = v
	}
	if v, err := s.run(ctx, otctlCmdExtAddr); err != nil {
		errs = append(errs, err)
	} else {
		backup.ExtAddress = v
	}

	active, err := s.GetActiveDataset(ctx)
	if err != nil {
		return backup, fmt.Errorf("active dataset: %w", err)
	}
	backup.ActiveDataset = strings.TrimSpace(active)

	pending, err := s.GetPendingDataset(ctx)
	if err == nil {
		backup.PendingDataset = strings.TrimSpace(pending)
	}

	if len(errs) > 0 {
		return backup, fmt.Errorf("backup metadata: %w", errs[0])
	}
	return backup, nil
}

// ImportBackup restores network credentials from a backup bundle.
func (s *ThreadService) ImportBackup(ctx context.Context, backup ConfigBackup) error {
	active := strings.TrimSpace(backup.ActiveDataset)
	if err := s.SetActiveDataset(ctx, active); err != nil {
		return fmt.Errorf("set active dataset: %w", err)
	}

	pending := strings.TrimSpace(backup.PendingDataset)
	if pending != "" {
		if err := s.SetPendingDataset(ctx, pending); err != nil {
			return fmt.Errorf("set pending dataset: %w", err)
		}
	}
	return nil
}

func parseConfigBackup(r *http.Request) (ConfigBackup, error) {
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

func validateConfigBackup(backup ConfigBackup) error {
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

func backupDir(stateDir string) string {
	return filepath.Join(stateDir, "backups")
}

func validateBackupFilename(name string) error {
	name = filepath.Base(name)
	if name == "" || name == "." || strings.Contains(name, "..") {
		return fmt.Errorf("invalid backup filename")
	}
	return nil
}

func readStoredBackup(stateDir, name string) ([]byte, error) {
	if err := validateBackupFilename(name); err != nil {
		return nil, err
	}
	path := filepath.Join(backupDir(stateDir), name)
	//nolint:gosec // G304: filename validated above to exclude path traversal
	return os.ReadFile(path)
}
