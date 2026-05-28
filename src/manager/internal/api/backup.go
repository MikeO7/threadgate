package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s *Server) handleBackupExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	backup, err := s.threads.ExportBackup(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export backup: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("threadgate-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := json.NewEncoder(w).Encode(backup); err != nil {
		log.Printf("[API Server] Failed to encode backup: %v\n", err)
	}
}

func (s *Server) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	backup, err := parseConfigBackup(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validateConfigBackup(backup); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.threads.ImportBackup(r.Context(), backup); err != nil {
		http.Error(w, fmt.Sprintf("Failed to import backup: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		jsonKeyStatus: jsonStatusOK,
		"message":     "Network credentials restored from backup",
	})
}

func (s *Server) handleBackupSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateDir == "" {
		http.Error(w, "Backup storage is not configured", http.StatusServiceUnavailable)
		return
	}

	backup, err := s.threads.ExportBackup(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export backup: %v", err), http.StatusInternalServerError)
		return
	}

	dir := backupDir(s.stateDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create backup directory: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("threadgate-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode backup: %v", err), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write backup: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		jsonKeyStatus: jsonStatusOK,
		"filename":    filename,
		"path":        path,
	})
}

func (s *Server) handleBackupFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateDir == "" {
		http.Error(w, "Backup storage is not configured", http.StatusServiceUnavailable)
		return
	}

	dir := backupDir(s.stateDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]string{})
			return
		}
		http.Error(w, fmt.Sprintf("Failed to list backups: %v", err), http.StatusInternalServerError)
		return
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		files = append(files, e.Name())
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		log.Printf("[API Server] Failed to encode backup list: %v\n", err)
	}
}

func (s *Server) handleBackupFileGet(w http.ResponseWriter, r *http.Request, name string) {
	data, err := readStoredBackup(s.stateDir, name)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(err.Error(), "invalid backup filename") {
			http.Error(w, "Invalid backup filename", http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to read backup: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	if _, err := w.Write(data); err != nil { //nolint:gosec // G705: data is a validated on-disk backup read for download
		log.Printf("[API Server] Failed to write backup file: %v\n", err)
	}
}

func (s *Server) handleBackupFileRestore(w http.ResponseWriter, r *http.Request, name string) {
	data, err := readStoredBackup(s.stateDir, name)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		if strings.Contains(err.Error(), "invalid backup filename") {
			http.Error(w, "Invalid backup filename", http.StatusBadRequest)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to read backup: %v", err), http.StatusInternalServerError)
		return
	}
	var backup ConfigBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		http.Error(w, fmt.Sprintf("Invalid backup file: %v", err), http.StatusBadRequest)
		return
	}
	if backup.Version == 0 {
		backup.Version = backupVersion
	}
	if err := validateConfigBackup(backup); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.threads.ImportBackup(r.Context(), backup); err != nil {
		http.Error(w, fmt.Sprintf("Failed to import backup: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		jsonKeyStatus: jsonStatusOK,
		"message":     fmt.Sprintf("Restored from %s", name),
	})
}

func (s *Server) handleBackupFile(w http.ResponseWriter, r *http.Request) {
	if s.stateDir == "" {
		http.Error(w, "Backup storage is not configured", http.StatusServiceUnavailable)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/backup/files/")
	name = filepath.Base(name)
	if err := validateBackupFilename(name); err != nil {
		http.Error(w, "Invalid backup filename", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleBackupFileGet(w, r, name)
	case http.MethodPost:
		s.handleBackupFileRestore(w, r, name)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/backup/import":
		s.handleBackupImport(w, r)
	case r.URL.Path == "/api/backup/save":
		s.handleBackupSave(w, r)
	case r.URL.Path == "/api/backup/files":
		s.handleBackupFiles(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/backup/files/"):
		s.handleBackupFile(w, r)
	case r.URL.Path == "/api/backup":
		s.handleBackupExport(w, r)
	default:
		http.NotFound(w, r)
	}
}
