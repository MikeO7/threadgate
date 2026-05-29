package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/pairing"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	var status runtime.Status
	if s.statusReporter != nil {
		status = s.statusReporter.GetStatus()
	}
	model := s.snapSvc.BuildDashboard(r.Context(), s.port, s.env.IsMock(), status, s.hassClient.Enabled(), s.hassClient.URL())
	view := NewDashboardView(model)
	tmpl := template.Must(template.New("dashboard").Funcs(template.FuncMap{
		"isLowBattery": func(battery string) bool {
			var val int
			if _, err := fmt.Sscanf(battery, "%d", &val); err == nil {
				return val < 15
			}
			return false
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
		},
	}).Parse(dashboardHTML))
	if err := tmpl.Execute(w, view); err != nil {
		log.Printf("[API Server] Failed to execute template: %v\n", err)
	}
}

func (s *Server) handleLogo(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(logoSVG))
}

func (s *Server) handleActiveDatasetDecode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	dataset, err := s.threads.GetActiveDataset(r.Context())
	if err != nil {
		log.Printf("[API Server] getActiveDataset failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to fetch active dataset: %v", err), http.StatusInternalServerError)
		return
	}
	writeDecodedDataset(w, dataset)
}

func (s *Server) handlePendingDatasetDecode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	dataset, err := s.threads.GetPendingDataset(r.Context())
	if err != nil {
		log.Printf("[API Server] getPendingDataset failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to fetch pending dataset: %v", err), http.StatusInternalServerError)
		return
	}
	writeDecodedDataset(w, dataset)
}

func (s *Server) handleDatasetDecode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	ds, err := thread.ParseDatasetHTTPBody(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	writeDecodedDataset(w, ds.Hex())
}

func writeDecodedDataset(w http.ResponseWriter, hexStr string) {
	ds, err := thread.ParseOperationalDatasetHex(hexStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode dataset: %v", err), http.StatusBadRequest)
		return
	}
	decoded, err := ds.Decode()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode dataset: %v", err), http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(decoded)
}

func (s *Server) handlePairInitiate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var req struct {
		AppName   string `json:"app_name"`
		AppURL    string `json:"app_url"`
		HassToken string `json:"hass_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.AppName == "" || req.AppURL == "" || req.HassToken == "" {
		http.Error(w, "Missing app_name, app_url, or hass_token", http.StatusBadRequest)
		return
	}
	p := s.pairMgr.AddRequest(req.AppName, req.AppURL, req.HassToken)
	_ = json.NewEncoder(w).Encode(p)
}

func (s *Server) handlePairStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	req, ok := s.pairMgr.GetRequest(id)
	if !ok {
		_ = json.NewEncoder(w).Encode(map[string]string{jsonKeyStatus: "expired"})
		return
	}

	resp := map[string]string{jsonKeyStatus: req.Status}
	if req.Status == pairing.StatusApproved {
		resp["message"] = "pairing approved successfully"
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handlePairActive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	reqs := s.pairMgr.GetActiveRequests()
	if reqs == nil {
		reqs = []*pairing.Request{}
	}
	_ = json.NewEncoder(w).Encode(reqs)
}

func (s *Server) handlePairApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		ID string `json:"pairing_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := s.pairMgr.ApproveAndSave(body.ID, s.env.Config.StateDir)
	if err != nil {
		if errors.Is(err, pairing.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		log.Printf("[API Server] Failed to save HASS config: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.hassClient.Reload(req.AppURL, req.HassToken)

	_ = json.NewEncoder(w).Encode(map[string]string{jsonKeyStatus: "approved"})
}

func (s *Server) handlePairDeny(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		ID string `json:"pairing_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ok := s.pairMgr.Deny(body.ID); !ok {
		http.Error(w, "Pairing request expired or not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{jsonKeyStatus: "denied"})
}
