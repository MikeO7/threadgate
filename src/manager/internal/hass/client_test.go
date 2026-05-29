package hass

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestClientDisabled(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeModeHardware,
	}
	client := NewClient(cfg)
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

var testMockDevices = []struct {
	Name        string  `json:"name"`
	Connections [][]any `json:"connections"`
}{
	{
		Name: "Kitchen Temp Sensor",
		Connections: [][]any{
			{"zigbee", "11:22:33:44:55:66:77:01"},
		},
	},
	{
		Name: "Living Room Blind",
		Connections: [][]any{
			{"mac", "1122334455667702"},
		},
	},
	{
		Name: "Other Connection",
		Connections: [][]any{
			{"ip", "192.168.1.1"},
		},
	},
}

func TestClientFetchViaTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			return
		}
		if r.Header.Get("Authorization") != "Bearer supersecret" {
			t.Errorf("Expected Auth header 'Bearer supersecret', got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testMockDevices)
	}))
	defer server.Close()

	cfg := &config.Config{
		HassURL:   server.URL,
		HassToken: "supersecret",
		Runtime:   config.RuntimeModeHardware,
	}

	client := NewClient(cfg)
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

	if names["1122334455667701"].Name != "Kitchen Temp Sensor" {
		t.Errorf("Expected Kitchen Temp Sensor, got %q", names["1122334455667701"].Name)
	}

	if names["1122334455667702"].Name != "Living Room Blind" {
		t.Errorf("Expected Living Room Blind, got %q", names["1122334455667702"].Name)
	}
}

func TestClientMockMode(t *testing.T) {
	cfg := &config.Config{
		Runtime: config.RuntimeModeFromMock(true),
	}
	client := NewClient(cfg)
	names, err := client.FetchDeviceNames(context.Background())
	if err != nil {
		t.Fatalf("FetchDeviceNames failed: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected mock device names")
	}
	status, errMsg := client.Status()
	if status != StatusMock || errMsg != "" {
		t.Fatalf("status = %q err = %q", status, errMsg)
	}
}

func TestMapConnection(t *testing.T) {
	mac, ok := mapConnection("thread", "11:22:33:44:55:66:77:88")
	if !ok || mac != "1122334455667788" {
		t.Fatalf("mapConnection thread: ok=%v mac=%q", ok, mac)
	}
	if _, ok := mapConnection("ip", "192.168.1.1"); ok {
		t.Fatal("expected ip connection to be ignored")
	}
}
