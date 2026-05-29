package thread

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// OperationalDataset is a validated Thread operational dataset TLV hex string.
type OperationalDataset struct {
	hex string
}

type datasetPayload struct {
	ActiveDataset      string `json:"ActiveDataset"`
	ActiveDatasetTlvs  string `json:"ActiveDatasetTlvs"`
	PendingDataset     string `json:"PendingDataset"`
	PendingDatasetTlvs string `json:"PendingDatasetTlvs"`
}

// ParseOperationalDatasetHex validates and wraps a hex-encoded TLV dataset.
func ParseOperationalDatasetHex(s string) (OperationalDataset, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return OperationalDataset{}, fmt.Errorf("empty dataset hex")
	}
	if _, err := hex.DecodeString(s); err != nil {
		return OperationalDataset{}, fmt.Errorf("invalid hex-encoded TLV string: %w", err)
	}
	return OperationalDataset{hex: s}, nil
}

// ParseDatasetHTTPBody extracts a dataset hex string from OTBR/HA JSON or raw hex bodies.
func ParseDatasetHTTPBody(body []byte) (OperationalDataset, error) {
	bodyStr := strings.TrimSpace(string(body))
	if bodyStr == "" {
		return OperationalDataset{}, fmt.Errorf("empty request body")
	}

	var payload datasetPayload
	if err := json.Unmarshal(body, &payload); err == nil {
		if hexVal := extractTLVFromPayload(payload); hexVal != "" {
			return ParseOperationalDatasetHex(hexVal)
		}
	}

	bodyStr = strings.Trim(bodyStr, `"'`)
	return ParseOperationalDatasetHex(bodyStr)
}

func extractTLVFromPayload(payload datasetPayload) string {
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

// Hex returns the raw TLV hex string.
func (d OperationalDataset) Hex() string {
	return d.hex
}

// Decode parses TLV fields into a human-readable structure.
func (d OperationalDataset) Decode() (DecodedDataset, error) {
	return DecodeDataset(d.hex)
}

// ContainsInsecureKey reports whether the TLV embeds Home Assistant's flagged default network key.
func (d OperationalDataset) ContainsInsecureKey() bool {
	return DatasetContainsInsecureNetworkKey(d.hex)
}

// IsValidDatasetHex reports whether s is non-empty valid hex suitable for a TLV dataset.
func IsValidDatasetHex(s string) bool {
	_, err := ParseOperationalDatasetHex(s)
	return err == nil
}
