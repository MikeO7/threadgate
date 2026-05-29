package otbrapi

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

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
