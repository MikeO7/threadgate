package thread

import (
	"encoding/json"
	"strings"
	"testing"
)

const testValidHex = "0e080000000000000001"

func TestParseOperationalDatasetHex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid hex",
			input:   testValidHex,
			wantErr: false,
		},
		{
			name:    "valid hex with whitespace",
			input:   "  " + testValidHex + "  ",
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

func TestParseDatasetHTTPBody_Basic(t *testing.T) {
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
			body:    []byte(testValidHex),
			wantHex: testValidHex,
			wantErr: false,
		},
		{
			name:    "raw hex in quotes",
			body:    []byte(`"` + testValidHex + `"`),
			wantHex: testValidHex,
			wantErr: false,
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

func buildJSONPayload(p datasetPayload) []byte {
	b, _ := json.Marshal(p)
	return b
}

func TestParseDatasetHTTPBody_JSON(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wantHex string
		wantErr bool
	}{
		{
			name:    "JSON with ActiveDatasetTlvs",
			body:    buildJSONPayload(datasetPayload{ActiveDatasetTlvs: testValidHex}),
			wantHex: testValidHex,
			wantErr: false,
		},
		{
			name:    "JSON with ActiveDataset",
			body:    buildJSONPayload(datasetPayload{ActiveDataset: testValidHex}),
			wantHex: testValidHex,
			wantErr: false,
		},
		{
			name:    "JSON with PendingDatasetTlvs",
			body:    buildJSONPayload(datasetPayload{PendingDatasetTlvs: testValidHex}),
			wantHex: testValidHex,
			wantErr: false,
		},
		{
			name:    "JSON with PendingDataset",
			body:    buildJSONPayload(datasetPayload{PendingDataset: testValidHex}),
			wantHex: testValidHex,
			wantErr: false,
		},
		{
			name:    "JSON with empty values",
			body:    buildJSONPayload(datasetPayload{ActiveDataset: "  "}),
			wantErr: true,
		},
		{
			name:    "JSON with no useful fields",
			body:    []byte(`{"some_other_field": "123"}`),
			wantErr: true,
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
	ds := OperationalDataset{hex: testValidHex}
	if got := ds.Hex(); got != testValidHex {
		t.Errorf("OperationalDataset.Hex() = %v, want %v", got, testValidHex)
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
			input: testValidHex,
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
