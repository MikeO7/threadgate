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
	log.Println("[API Server] Initiating thread network backup export...")
	backup, err := s.backup.Export(r.Context())
	if err != nil {
		log.Printf("[API Server] Backup export failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to export backup: %v", err), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("threadgate-backup-%s.json", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	if err := json.NewEncoder(w).Encode(backup); err != nil {
		log.Printf("[API Server] Failed to encode backup: %v\n", err)
	} else {
		log.Printf("[API Server] Backup export completed successfully: %s\n", filename)
	}
}

func (s *Server) handleBackupImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	log.Println("[API Server] Initiating thread network backup import from request payload...")
	backup, err := ParseConfigBackupRequest(r)
	if err != nil {
		log.Printf("[API Server] Backup import failed: invalid request body structure: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := ValidateConfigBackup(backup); err != nil {
		log.Printf("[API Server] Backup import rejected: validation failed: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.backup.Import(r.Context(), backup); err != nil {
		log.Printf("[API Server] Backup import failed during restoration to OTBR: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to import backup: %v", err), http.StatusInternalServerError)
		return
	}

	log.Println("[API Server] Backup import successfully restored credentials to OTBR.")
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
	if s.backup.stateDir == "" {
		log.Println("[API Server] Backup save failed: backup storage is not configured")
		http.Error(w, "Backup storage is not configured", http.StatusServiceUnavailable)
		return
	}

	log.Printf("[API Server] Saving backup to disk state directory %s...\n", s.backup.stateDir)
	filename, path, err := s.backup.Save(r.Context())
	if err != nil {
		log.Printf("[API Server] Backup save to disk failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to save backup: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[API Server] Backup successfully saved to disk: %s\n", path)
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
	if s.backup.stateDir == "" {
		http.Error(w, "Backup storage is not configured", http.StatusServiceUnavailable)
		return
	}

	files, err := s.backup.ListFiles()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list backups: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(files); err != nil {
		log.Printf("[API Server] Failed to encode backup list: %v\n", err)
	}
}

func (s *Server) handleBackupFileGet(w http.ResponseWriter, r *http.Request, name string) {
	data, err := s.backup.ReadFile(name)
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
	//nolint:gosec // G706: filename is validated and safe
	log.Printf("[API Server] Initiating restore from backup file: %s...\n", name)
	data, err := s.backup.ReadFile(name)
	if err != nil {
		if os.IsNotExist(err) {
			//nolint:gosec // G706: filename is validated and safe
			log.Printf("[API Server] Restore failed: backup file %s does not exist\n", name)
			http.NotFound(w, r)
			return
		}
		if strings.Contains(err.Error(), "invalid backup filename") {
			//nolint:gosec // G706: filename is validated and safe
			log.Printf("[API Server] Restore failed: invalid backup filename: %s\n", name)
			http.Error(w, "Invalid backup filename", http.StatusBadRequest)
			return
		}
		//nolint:gosec // G706: filename is validated and safe
		log.Printf("[API Server] Restore failed: error reading file %s: %v\n", name, err)
		http.Error(w, fmt.Sprintf("Failed to read backup: %v", err), http.StatusInternalServerError)
		return
	}
	var backup ConfigBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		log.Printf("[API Server] Restore failed: backup file content is not valid JSON: %v\n", err)
		http.Error(w, fmt.Sprintf("Invalid backup file: %v", err), http.StatusBadRequest)
		return
	}
	if backup.Version == 0 {
		backup.Version = backupVersion
	}
	if err := ValidateConfigBackup(backup); err != nil {
		log.Printf("[API Server] Restore failed: backup validation failed: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.backup.Import(r.Context(), backup); err != nil {
		log.Printf("[API Server] Restore failed: failed to import backup into OTBR: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to import backup: %v", err), http.StatusInternalServerError)
		return
	}
	//nolint:gosec // G706: filename is validated and safe
	log.Printf("[API Server] Successfully restored credentials from backup file: %s\n", name)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		jsonKeyStatus: jsonStatusOK,
		"message":     fmt.Sprintf("Restored from %s", name),
	})
}

func (s *Server) handleBackupFile(w http.ResponseWriter, r *http.Request) {
	if s.backup.stateDir == "" {
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
