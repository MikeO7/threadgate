package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

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

func (s *Server) handleChannelScan(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	results, err := s.threads.ScanChannels(r.Context())
	if err != nil {
		log.Printf("[API Server] ChannelScan failed: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(results); err != nil {
		log.Printf("[API Server] Failed to encode channel scan results: %v\n", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "application/json")
	if s.statusReporter == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{jsonKeyStatus: "unknown"})
		return
	}
	status := s.statusReporter.GetStatus()
	report := s.readiness.Check()
	payload := struct {
		runtime.Status
		Readiness radio.ReadinessReport `json:"readiness"`
	}{
		Status:    status,
		Readiness: report,
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("[API Server] Failed to encode health: %v\n", err)
	}
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

func (s *Server) handleHealthcheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	summary, err := s.threads.CheckHealth(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(summary)
}

func (s *Server) handleTopology(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enriched := s.snapSvc.Build(r.Context())
	if err := json.NewEncoder(w).Encode(enriched.Snapshot); err != nil {
		log.Printf("[API Server] Failed to encode topology: %v\n", err)
	}
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
		jsonKeyStatus: "success",
		"path":         tracePath,
		jsonKeyMessage: "Continuous execution trace flushed successfully",
	})
}
