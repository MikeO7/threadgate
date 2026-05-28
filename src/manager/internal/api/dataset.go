package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type DatasetPayload struct {
	ActiveDataset      string `json:"ActiveDataset"`
	ActiveDatasetTlvs  string `json:"ActiveDatasetTlvs"`
	PendingDataset     string `json:"PendingDataset"`
	PendingDatasetTlvs string `json:"PendingDatasetTlvs"`
}

func parseDatasetHex(r *http.Request) (string, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}
	defer func() {
		_ = r.Body.Close()
	}()

	bodyStr := strings.TrimSpace(string(bodyBytes))
	if bodyStr == "" {
		return "", fmt.Errorf("empty request body")
	}

	var payload DatasetPayload
	if err := json.Unmarshal(bodyBytes, &payload); err == nil {
		if hexVal := extractTLVFromPayload(payload); hexVal != "" {
			return hexVal, nil
		}
	}

	// Fallback to treating body as raw hex string.
	bodyStr = strings.Trim(bodyStr, `"'`)
	return bodyStr, nil
}

func extractTLVFromPayload(payload DatasetPayload) string {
	if payload.ActiveDatasetTlvs != "" {
		return strings.TrimSpace(payload.ActiveDatasetTlvs)
	}
	if payload.ActiveDataset != "" {
		return strings.TrimSpace(payload.ActiveDataset)
	}
	if payload.PendingDatasetTlvs != "" {
		return strings.TrimSpace(payload.PendingDatasetTlvs)
	}
	if payload.PendingDataset != "" {
		return strings.TrimSpace(payload.PendingDataset)
	}
	return ""
}

func isValidHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil && len(s) > 0
}
