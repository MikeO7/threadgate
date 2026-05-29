package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

func TestParseDatasetHTTPBody(t *testing.T) {
	body := []byte(`{"ActiveDatasetTlvs":"` + activeDatasetHex + `"}`)
	ds, err := thread.ParseDatasetHTTPBody(body)
	if err != nil {
		t.Fatalf("ParseDatasetHTTPBody failed: %v", err)
	}
	if ds.Hex() != activeDatasetHex {
		t.Errorf("got %q want %q", ds.Hex(), activeDatasetHex)
	}

	gotRaw, err := thread.ParseDatasetHTTPBody([]byte(activeDatasetHex))
	if err != nil {
		t.Fatalf("ParseDatasetHTTPBody raw failed: %v", err)
	}
	if gotRaw.Hex() != activeDatasetHex {
		t.Errorf("got %q want %q", gotRaw.Hex(), activeDatasetHex)
	}

	if _, err := thread.ParseDatasetHTTPBody(nil); err == nil {
		t.Error("expected error for empty body")
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/", strings.NewReader(`"`+activeDatasetHex+`"`))
	bodyBytes, _ := io.ReadAll(req.Body)
	gotQuoted, err := thread.ParseDatasetHTTPBody(bodyBytes)
	if err != nil {
		t.Fatalf("expected quoted fallback parse, got error: %v", err)
	}
	if gotQuoted.Hex() != activeDatasetHex {
		t.Fatalf("unexpected fallback body: %q", gotQuoted.Hex())
	}
}

func TestIsValidDatasetHex(t *testing.T) {
	if !thread.IsValidDatasetHex(activeDatasetHex) {
		t.Error("expected valid hex")
	}
	if thread.IsValidDatasetHex("not-hex") {
		t.Error("expected invalid hex")
	}
	if thread.IsValidDatasetHex("") {
		t.Error("expected empty string invalid")
	}
}

func TestRunSnapshot(t *testing.T) {
	snap, err := RunSnapshot(context.Background(), thread.NewClient(thread.NewMock(), thread.PolicyBestEffort))
	if err != nil {
		t.Fatalf("RunSnapshot failed: %v", err)
	}
	if snap.NetworkName != mockNetworkName {
		t.Errorf("unexpected network name: %q", snap.NetworkName)
	}
}
