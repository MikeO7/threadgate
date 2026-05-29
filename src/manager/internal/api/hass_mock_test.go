package api

import (
	"context"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestHassClientMockMode(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeModeMock,
	}

	client := NewHassClient(cfg)
	if !client.Enabled() {
		t.Error("Expected client to be enabled in mock mode")
	}

	names, err := client.FetchDeviceNames(context.Background())
	if err != nil {
		t.Fatalf("FetchDeviceNames in mock mode returned error: %v", err)
	}

	if len(names) == 0 {
		t.Error("Expected mapped mock names, got 0")
	}

	if names["0000000000000001"] != "Living Room Multi-Sensor" {
		t.Errorf("Expected 'Living Room Multi-Sensor', got %q", names["0000000000000001"])
	}

	if names["1122334455667788"] != "ThreadGate Border Router" {
		t.Errorf("Expected 'ThreadGate Border Router', got %q", names["1122334455667788"])
	}
}
