package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestHassClientDisabled(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeModeHardware,
	}
	client := NewHassClient(cfg)
	if client.Enabled() {
		t.Error("Expected client to be disabled with empty config")
	}

	names, err := client.FetchDeviceNames(context.Background())
	if err != nil {
		t.Errorf("FetchDeviceNames returned error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Expected empty names, got %d", len(names))
	}
}

func TestHassClientFetch(t *testing.T) {
	mockDevices := []hassDevice{
		{
			Name: "Kitchen Temp Sensor",
			Connections: [][]interface{}{
				{"zigbee", "11:22:33:44:55:66:77:01"},
			},
		},
		{
			Name: "Living Room Blind",
			Connections: [][]interface{}{
				{"mac", "1122334455667702"},
			},
		},
		{
			Name: "Other Connection",
			Connections: [][]interface{}{
				{"ip", "192.168.1.1"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer supersecret" {
			t.Errorf("Expected Auth header 'Bearer supersecret', got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockDevices)
	}))
	defer server.Close()

	cfg := &config.Config{
		HassURL:   server.URL,
		HassToken: "supersecret",
		Runtime:   config.RuntimeModeHardware,
	}

	client := NewHassClient(cfg)
	if !client.Enabled() {
		t.Error("Expected client to be enabled")
	}

	names, err := client.FetchDeviceNames(context.Background())
	if err != nil {
		t.Fatalf("FetchDeviceNames failed: %v", err)
	}

	if len(names) != 2 {
		t.Errorf("Expected 2 mapped names, got %d", len(names))
	}

	if names["1122334455667701"] != "Kitchen Temp Sensor" {
		t.Errorf("Expected Kitchen Temp Sensor, got %q", names["1122334455667701"])
	}

	if names["1122334455667702"] != "Living Room Blind" {
		t.Errorf("Expected Living Room Blind, got %q", names["1122334455667702"])
	}
}
