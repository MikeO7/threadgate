// Package otbrapi serves Home Assistant OTBR REST compatibility endpoints.
package otbrapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
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
