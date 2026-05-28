// Package api exposes REST APIs and sleek monitoring dashboards for the ThreadGate ecosystem.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// Server handles REST API and diagnostic web requests.
type Server struct {
	port           int
	env            *env.Env
	threads        *thread.Client
	backup         *BackupStore
	mu             sync.Mutex
	srv            *http.Server
	statusReporter runtime.Reporter
}

// NewServer creates a server wired to the composition root Env.
func NewServer(e *env.Env, port int, stateDir string) *Server {
	var reporter runtime.Reporter
	if e.Status != nil {
		reporter = e.Status
	}
	return &Server{
		port:           port,
		env:            e,
		threads:        e.Thread,
		backup:         NewBackupStore(e.Thread, stateDir),
		statusReporter: reporter,
	}
}

// NewServerWithOtCtl is a convenience constructor for tests and wiring.
func NewServerWithOtCtl(port int, otctl OtCtl, mockMode bool) *Server {
	return NewServer(testEnv(otctl, mockMode), port, "")
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
		{"/api/trace/flush", s.handleTraceFlush},
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
	return s.LoggingMiddleware(mux)
}

// Start launches the HTTP listener.
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()

	log.Printf("[API Server] Exposing REST API and modern dashboard on port %d...\n", s.port)
	return srv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP listener.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info, err := s.threads.NodeInfo(r.Context())
	if err != nil {
		log.Printf("[API Server] NodeInfo failed: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.Printf("[API Server] Failed to encode response: %v\n", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	_ = r
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
		log.Printf("[API Server] getActiveDataset failed: %v\n", err)
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
		log.Printf("[API Server] setActiveDataset failed to parse request body: %v\n", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if !isValidHex(hexStr) {
		log.Printf("[API Server] setActiveDataset rejected: invalid dataset format (must be a hex-encoded TLV string)\n")
		http.Error(w, "Invalid dataset format: must be a hex-encoded TLV string", http.StatusBadRequest)
		return
	}
	log.Printf("[API Server] Attempting to update Active Dataset to hex: %s...\n", hexStr)
	if err := s.threads.SetActiveDataset(r.Context(), hexStr); err != nil {
		log.Printf("[API Server] Failed to update Active Dataset in OTBR: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to set active dataset: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("[API Server] Active Dataset successfully updated in OTBR.")
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
		log.Printf("[API Server] getPendingDataset failed: %v\n", err)
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
		log.Printf("[API Server] setPendingDataset failed to parse request body: %v\n", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	if !isValidHex(hexStr) {
		log.Printf("[API Server] setPendingDataset rejected: invalid dataset format (must be a hex-encoded TLV string)\n")
		http.Error(w, "Invalid dataset format: must be a hex-encoded TLV string", http.StatusBadRequest)
		return
	}
	log.Printf("[API Server] Attempting to update Pending Dataset to hex: %s...\n", hexStr)
	if err := s.threads.SetPendingDataset(r.Context(), hexStr); err != nil {
		log.Printf("[API Server] Failed to update Pending Dataset in OTBR: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to set pending dataset: %v", err), http.StatusInternalServerError)
		return
	}
	log.Println("[API Server] Pending Dataset successfully updated in OTBR.")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Pending dataset successfully updated"))
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	diag, err := s.threads.Diagnostics(r.Context())
	if err != nil {
		log.Printf("[API Server] Diagnostics failed: %v\n", err)
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

	var status runtime.Status
	if s.statusReporter != nil {
		status = s.statusReporter.GetStatus()
	}
	view := NewDashboardView(snap, s.port, s.env.IsMock(), status)
	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))
	if err := tmpl.Execute(w, view); err != nil {
		log.Printf("[API Server] Failed to execute template: %v\n", err)
	}
}

// RunSnapshot is a test helper that builds a snapshot with a timeout context.
func RunSnapshot(ctx context.Context, threads *thread.Client) (topology.Snapshot, error) {
	return threads.BuildSnapshot(ctx)
}

func (s *Server) handleTraceFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	tracePath := "/tmp/threadgate.trace"
	if s.env.Config.StateDir != "" {
		tracePath = fmt.Sprintf("%s/threadgate-%d.trace", s.env.Config.StateDir, time.Now().Unix())
	}

	err := runtime.GlobalFlightRecorder.Flush(tracePath)
	if err != nil {
		log.Printf("[API Server] Failed to flush flight recorder trace: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"path":    tracePath,
		"message": "Continuous execution trace flushed successfully", //nolint:goconst
	})
}
