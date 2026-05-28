// Package api exposes REST APIs and sleek monitoring dashboards for the ThreadGate ecosystem.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/api/topology"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

// Server handles REST API and diagnostic web requests.
type Server struct {
	port           int
	mockMode       bool
	threads        *ThreadService
	backup         *BackupStore
	srv            *http.Server
	statusReporter runtime.Reporter
}

// NewServer creates a server wired to the given ThreadService.
func NewServer(port int, threads *ThreadService, mockMode bool, stateDir string, statusReporter runtime.Reporter) *Server {
	return &Server{
		port:           port,
		threads:        threads,
		mockMode:       mockMode,
		backup:         NewBackupStore(threads, stateDir),
		statusReporter: statusReporter,
	}
}

// NewServerWithOtCtl is a convenience constructor for tests and wiring.
func NewServerWithOtCtl(port int, otctl OtCtl, mockMode bool) *Server {
	return NewServer(port, NewThreadService(otctl, CollectBestEffort), mockMode, "", nil)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	routes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"/api/node", s.handleNodeInfo},
		{"/api/health", s.handleHealth},
		{"/node/dataset/active", s.handleActiveDataset},
		{"/api/node/dataset/active", s.handleActiveDataset},
		{"/node/dataset/pending", s.handlePendingDataset},
		{"/api/node/dataset/pending", s.handlePendingDataset},
		{"/api/diagnostics", s.handleDiagnostics},
		{"/api/topology", s.handleTopology},
		{"/api/backup/import", s.handleBackup},
		{"/api/backup/save", s.handleBackup},
		{"/api/backup/files/", s.handleBackup},
		{"/api/backup/files", s.handleBackup},
		{"/api/backup", s.handleBackup},
		{"/", s.handleDashboard},
	}
	for _, route := range routes {
		mux.HandleFunc(route.pattern, route.handler)
	}
}

// Handler returns the HTTP handler for testing without binding a port.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return mux
}

// Start launches the HTTP listener.
func (s *Server) Start() error {
	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("[API Server] Exposing REST API and modern dashboard on port %d...\n", s.port)
	return s.srv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP listener.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info, err := s.threads.NodeInfo(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("[API Server] Failed to encode response: %v\n", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.statusReporter == nil {
		_ = json.NewEncoder(w).Encode(struct {
			Status string `json:"status"`
		}{Status: "unknown"})
		return
	}
	status := s.statusReporter.GetStatus()
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Printf("[API Server] Failed to encode health: %v\n", err)
	}
}

func (s *Server) handleActiveDataset(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getActiveDataset(w, r)
	case http.MethodPut:
		s.setActiveDataset(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getActiveDataset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	dataset, err := s.threads.GetActiveDataset(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch active dataset: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write([]byte(dataset)); err != nil {
		log.Printf("[API Server] Failed to write dataset: %v\n", err)
	}
}

func (s *Server) setActiveDataset(w http.ResponseWriter, r *http.Request) {
	hexStr, err := parseDatasetHex(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if !isValidHex(hexStr) {
		http.Error(w, "Invalid dataset format: must be a hex-encoded TLV string", http.StatusBadRequest)
		return
	}
	if err := s.threads.SetActiveDataset(r.Context(), hexStr); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set active dataset: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Active dataset successfully updated"))
}

func (s *Server) handlePendingDataset(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getPendingDataset(w, r)
	case http.MethodPut:
		s.setPendingDataset(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getPendingDataset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	dataset, err := s.threads.GetPendingDataset(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch pending dataset: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write([]byte(dataset)); err != nil {
		log.Printf("[API Server] Failed to write pending dataset: %v\n", err)
	}
}

func (s *Server) setPendingDataset(w http.ResponseWriter, r *http.Request) {
	hexStr, err := parseDatasetHex(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if !isValidHex(hexStr) {
		http.Error(w, "Invalid dataset format: must be a hex-encoded TLV string", http.StatusBadRequest)
		return
	}
	if err := s.threads.SetPendingDataset(r.Context(), hexStr); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set pending dataset: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Pending dataset successfully updated"))
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	diag, err := s.threads.Diagnostics(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(diag); err != nil {
		log.Printf("[API Server] Failed to encode diagnostics: %v\n", err)
	}
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	snap, err := s.threads.BuildSnapshot(r.Context())
	if err != nil {
		log.Printf("[API Server] Topology snapshot partial: %v\n", err)
	}
	if err := json.NewEncoder(w).Encode(snap); err != nil {
		log.Printf("[API Server] Failed to encode topology: %v\n", err)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	snap, err := s.threads.BuildSnapshot(r.Context())
	if err != nil {
		log.Printf("[API Server] Dashboard snapshot partial: %v\n", err)
	}

	view := NewDashboardView(snap, s.port, s.mockMode)
	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
	if err := tmpl.Execute(w, view); err != nil {
		log.Printf("[API Server] Failed to execute template: %v\n", err)
	}
}

// RunSnapshot is a test helper that builds a snapshot with a timeout context.
func RunSnapshot(ctx context.Context, threads *ThreadService) (topology.Snapshot, error) {
	return threads.BuildSnapshot(ctx)
}
