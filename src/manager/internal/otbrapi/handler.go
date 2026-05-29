// Package otbrapi serves Home Assistant OTBR REST compatibility endpoints.
package otbrapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
	"io"
	"log"
	"net/http"
	"strings"
)

const (
	nodeStateEnable  = "enable"
	nodeStateDisable = "disable"
)

// StatusReader supplies coprocessor version for OTBR REST compat.
type StatusReader interface {
	GetStatus() runtime.Status
}

// Handler serves Home Assistant OTBR REST compatibility endpoints.
type Handler struct {
	Ops      ThreadOps
	Status   StatusReader
	MockMode bool
}

func (h *Handler) HandleAPIActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{})
}

func (h *Handler) HandleBorderAgentID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := h.Ops.NodeInfo(r.Context())
	if err != nil {
		log.Printf("[OTBR] border agent id: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id := NormalizeHexID(info.ExtAddress)
	if id == "" {
		http.Error(w, "no extended address", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(id)
}

func (h *Handler) HandleExtAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := h.Ops.NodeInfo(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id := NormalizeHexID(info.ExtAddress)
	if id == "" {
		http.Error(w, "no extended address", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(id)
}

func (h *Handler) HandleCoprocessorVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	version := "ThreadGate/1.0.0"
	if h.Status != nil {
		if v := h.Status.GetStatus().ProbedVersion; v != "" {
			version = v
		}
	} else if h.MockMode {
		version = "ThreadGateMock/1.0.0; SIMULATION"
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(version); err != nil {
		log.Printf("[OTBR] coprocessor version encode: %v\n", err)
	}
}

func (h *Handler) HandleNodeState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	state := strings.Trim(strings.TrimSpace(string(body)), `"`)
	if state != nodeStateEnable && state != nodeStateDisable {
		http.Error(w, "expected enable or disable", http.StatusBadRequest)
		return
	}
	if err := h.Ops.SetNodeState(r.Context(), state == nodeStateEnable); err != nil {
		log.Printf("[OTBR] set node state %q: %v\n", state, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// NormalizeHexID strips 0x prefix and separators from a hex identifier.
func NormalizeHexID(s string) string {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "0x")
	s = strings.ReplaceAll(s, ":", "")
	return s
}

// ThreadOps is the seam for OTBR-compatible Thread operations.
type ThreadOps interface {
	NodeInfo(ctx context.Context) (thread.NodeInfo, error)
	GetActiveDataset(ctx context.Context) (string, error)
	SetActiveDataset(ctx context.Context, hexStr string) error
	GetPendingDataset(ctx context.Context) (string, error)
	SetPendingDataset(ctx context.Context, hexStr string) error
	SetNodeState(ctx context.Context, enable bool) error
}

// ClientAdapter satisfies ThreadOps using a thread.Client.
type ClientAdapter struct {
	Client *thread.Client
}

func (a *ClientAdapter) NodeInfo(ctx context.Context) (thread.NodeInfo, error) {
	return a.Client.NodeInfo(ctx)
}

func (a *ClientAdapter) GetActiveDataset(ctx context.Context) (string, error) {
	return a.Client.GetActiveDataset(ctx)
}

func (a *ClientAdapter) SetActiveDataset(ctx context.Context, hexStr string) error {
	return a.Client.SetActiveDataset(ctx, hexStr)
}

func (a *ClientAdapter) GetPendingDataset(ctx context.Context) (string, error) {
	return a.Client.GetPendingDataset(ctx)
}

func (a *ClientAdapter) SetPendingDataset(ctx context.Context, hexStr string) error {
	return a.Client.SetPendingDataset(ctx, hexStr)
}

func (a *ClientAdapter) SetNodeState(ctx context.Context, enable bool) error {
	return a.Client.SetNodeState(ctx, enable)
}

func (h *Handler) HandleActiveDataset(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getActiveDataset(w, r)
	case http.MethodPut:
		h.setActiveDataset(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) HandlePendingDataset(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getPendingDataset(w, r)
	case http.MethodPut:
		h.setPendingDataset(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getActiveDataset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	dataset, err := h.Ops.GetActiveDataset(r.Context())
	if err != nil {
		log.Printf("[OTBR] getActiveDataset failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to fetch active dataset: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write([]byte(dataset)); err != nil {
		log.Printf("[OTBR] write active dataset: %v\n", err)
	}
}

func (h *Handler) setActiveDataset(w http.ResponseWriter, r *http.Request) {
	ds, err := parseDatasetRequest(r)
	if err != nil {
		log.Printf("[OTBR] setActiveDataset parse: %v\n", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	log.Printf("[OTBR] Updating active dataset to hex: %s...\n", ds.Hex())
	if err := h.Ops.SetActiveDataset(r.Context(), ds.Hex()); err != nil {
		log.Printf("[OTBR] setActiveDataset failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to set active dataset: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Active dataset successfully updated"))
}

func (h *Handler) getPendingDataset(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	dataset, err := h.Ops.GetPendingDataset(r.Context())
	if err != nil {
		log.Printf("[OTBR] getPendingDataset failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to fetch pending dataset: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := w.Write([]byte(dataset)); err != nil {
		log.Printf("[OTBR] write pending dataset: %v\n", err)
	}
}

func (h *Handler) setPendingDataset(w http.ResponseWriter, r *http.Request) {
	ds, err := parseDatasetRequest(r)
	if err != nil {
		log.Printf("[OTBR] setPendingDataset parse: %v\n", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	log.Printf("[OTBR] Updating pending dataset to hex: %s...\n", ds.Hex())
	if err := h.Ops.SetPendingDataset(r.Context(), ds.Hex()); err != nil {
		log.Printf("[OTBR] setPendingDataset failed: %v\n", err)
		http.Error(w, fmt.Sprintf("Failed to set pending dataset: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Pending dataset successfully updated"))
}

func parseDatasetRequest(r *http.Request) (thread.OperationalDataset, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return thread.OperationalDataset{}, fmt.Errorf("read body: %w", err)
	}
	return thread.ParseDatasetHTTPBody(body)
}
