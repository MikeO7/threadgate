package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractTLVFromPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload DatasetPayload
		want    string
	}{
		{"active tlvs", DatasetPayload{ActiveDatasetTlvs: " aa "}, "aa"},
		{"active dataset", DatasetPayload{ActiveDataset: "bb"}, "bb"},
		{"pending tlvs", DatasetPayload{PendingDatasetTlvs: "cc"}, "cc"},
		{"pending dataset", DatasetPayload{PendingDataset: "dd"}, "dd"},
		{"empty", DatasetPayload{}, ""},
	}
	for _, tt := range tests {
		if got := extractTLVFromPayload(tt.payload); got != tt.want {
			t.Errorf("%s: got %q want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseDatasetHex(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"ActiveDatasetTlvs":"`+activeDatasetHex+`"}`))
	got, err := parseDatasetHex(req)
	if err != nil {
		t.Fatalf("parseDatasetHex failed: %v", err)
	}
	if got != activeDatasetHex {
		t.Errorf("got %q want %q", got, activeDatasetHex)
	}

	reqRaw := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(activeDatasetHex))
	gotRaw, err := parseDatasetHex(reqRaw)
	if err != nil {
		t.Fatalf("parseDatasetHex raw failed: %v", err)
	}
	if gotRaw != activeDatasetHex {
		t.Errorf("got %q want %q", gotRaw, activeDatasetHex)
	}

	reqEmpty := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(nil))
	if _, err := parseDatasetHex(reqEmpty); err == nil {
		t.Error("expected error for empty body")
	}
}

func TestIsValidHex(t *testing.T) {
	if !isValidHex(activeDatasetHex) {
		t.Error("expected valid hex")
	}
	if isValidHex("not-hex") {
		t.Error("expected invalid hex")
	}
	if isValidHex("") {
		t.Error("expected empty string invalid")
	}
}

func TestRunSnapshot(t *testing.T) {
	snap, err := RunSnapshot(context.Background(), NewThreadService(NewMockOtCtl(), CollectBestEffort))
	if err != nil {
		t.Fatalf("RunSnapshot failed: %v", err)
	}
	if snap.NetworkName != mockNetworkName {
		t.Errorf("unexpected network name: %q", snap.NetworkName)
	}
}
