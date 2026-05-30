package thread

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOperationalDatasetHex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid hex",
			input:   "0e080000000000000001",
			wantErr: false,
		},
		{
			name:    "valid hex with whitespace",
			input:   "  0e080000000000000001  ",
			wantErr: false,
		},
		{
			name:    "empty hex",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "invalid hex string",
			input:   "not-a-hex-string",
			wantErr: true,
		},
		{
			name:    "odd length hex",
			input:   "abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOperationalDatasetHex(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOperationalDatasetHex() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.Hex() != strings.TrimSpace(tt.input) {
				t.Errorf("ParseOperationalDatasetHex() got hex %v, want %v", got.Hex(), strings.TrimSpace(tt.input))
			}
		})
	}
}

func TestParseDatasetHTTPBody(t *testing.T) {
	validHex := "0e080000000000000001"

	tests := []struct {
		name    string
		body    []byte
		wantHex string
		wantErr bool
	}{
		{
			name:    "empty body",
			body:    []byte(""),
			wantErr: true,
		},
		{
			name:    "whitespace only",
			body:    []byte("   "),
			wantErr: true,
		},
		{
			name:    "raw hex",
			body:    []byte(validHex),
			wantHex: validHex,
			wantErr: false,
		},
		{
			name:    "raw hex in quotes",
			body:    []byte(`"` + validHex + `"`),
			wantHex: validHex,
			wantErr: false,
		},
		{
			name: "JSON with ActiveDatasetTlvs",
			body: func() []byte {
				payload := datasetPayload{ActiveDatasetTlvs: validHex}
				b, _ := json.Marshal(payload)
				return b
			}(),
			wantHex: validHex,
			wantErr: false,
		},
		{
			name: "JSON with ActiveDataset fallback",
			body: func() []byte {
				payload := datasetPayload{ActiveDataset: validHex}
				b, _ := json.Marshal(payload)
				return b
			}(),
			wantHex: validHex,
			wantErr: false,
		},
		{
			name: "JSON with PendingDatasetTlvs fallback",
			body: func() []byte {
				payload := datasetPayload{PendingDatasetTlvs: validHex}
				b, _ := json.Marshal(payload)
				return b
			}(),
			wantHex: validHex,
			wantErr: false,
		},
		{
			name: "JSON with PendingDataset fallback",
			body: func() []byte {
				payload := datasetPayload{PendingDataset: validHex}
				b, _ := json.Marshal(payload)
				return b
			}(),
			wantHex: validHex,
			wantErr: false,
		},
		{
			name: "JSON with empty string values fallback to raw parse error",
			body: func() []byte {
				payload := datasetPayload{ActiveDataset: "  "}
				b, _ := json.Marshal(payload)
				return b
			}(),
			wantErr: true, // "  " is invalid hex
		},
		{
			name: "JSON with no useful fields fallback to raw parse",
			body: []byte(`{"some_other_field": "123"}`),
			wantErr: true, // "{"some_other_field": "123"}" is invalid hex
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDatasetHTTPBody(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDatasetHTTPBody() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.Hex() != tt.wantHex {
				t.Errorf("ParseDatasetHTTPBody() got hex %v, want %v", got.Hex(), tt.wantHex)
			}
		})
	}
}

func TestOperationalDataset_Hex(t *testing.T) {
	hexVal := "0e080000000000000001"
	ds := OperationalDataset{hex: hexVal}
	if got := ds.Hex(); got != hexVal {
		t.Errorf("OperationalDataset.Hex() = %v, want %v", got, hexVal)
	}
}

func TestOperationalDataset_Decode(t *testing.T) {
	// We only test that Decode() delegates to DecodeDataset() properly.
	// DecodeDataset itself is tested in tlv_test.go.
	ds := OperationalDataset{hex: ValidOperationalDatasetHex}
	decoded, err := ds.Decode()
	if err != nil {
		t.Fatalf("OperationalDataset.Decode() failed: %v", err)
	}
	if decoded.NetworkName != TestNetworkName {
		t.Errorf("OperationalDataset.Decode() network name = %v, want %v", decoded.NetworkName, TestNetworkName)
	}
}

func TestIsValidDatasetHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid hex",
			input: "0e080000000000000001",
			want:  true,
		},
		{
			name:  "empty",
			input: "",
			want:  false,
		},
		{
			name:  "invalid hex",
			input: "not-hex",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidDatasetHex(tt.input); got != tt.want {
				t.Errorf("IsValidDatasetHex() = %v, want %v", got, tt.want)
			}
		})
	}
}
