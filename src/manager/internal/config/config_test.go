package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any existing env variables to ensure default behavior
	_ = os.Unsetenv("OTBR_RADIO_URL")
	_ = os.Unsetenv("OTBR_LOG_LEVEL")
	_ = os.Unsetenv("OTBR_AUTO_DISCOVER")
	_ = os.Unsetenv("OTBR_STATE_DIR")
	_ = os.Unsetenv("OTBR_BAUDRATE")
	_ = os.Unsetenv("OTBR_PORT")
	_ = os.Unsetenv("OTBR_MOCK_MODE")

	cfg := Load()

	if cfg.RadioURL != "" {
		t.Errorf("Expected empty RadioURL default, got %q", cfg.RadioURL)
	}
	if cfg.Baudrate != 460800 {
		t.Errorf("Expected Baudrate default 460800, got %d", cfg.Baudrate)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel default 'info', got %q", cfg.LogLevel)
	}
	if cfg.Port != 8081 {
		t.Errorf("Expected Port default 8081, got %d", cfg.Port)
	}
	if !cfg.AutoDiscover {
		t.Errorf("Expected AutoDiscover default true, got %t", cfg.AutoDiscover)
	}
	if cfg.StateDir != "/data" {
		t.Errorf("Expected StateDir default '/data', got %q", cfg.StateDir)
	}
	if cfg.MockMode {
		t.Errorf("Expected MockMode default false, got %t", cfg.MockMode)
	}
}

func TestLoadCustomValues(t *testing.T) {
	_ = os.Setenv("OTBR_RADIO_URL", "spinel+hdlc+uart:///dev/ttyUSB0")
	_ = os.Setenv("OTBR_LOG_LEVEL", "debug")
	_ = os.Setenv("OTBR_AUTO_DISCOVER", "false")
	_ = os.Setenv("OTBR_STATE_DIR", "/tmp/state")
	_ = os.Setenv("OTBR_BAUDRATE", "115200")
	_ = os.Setenv("OTBR_PORT", "9090")
	_ = os.Setenv("OTBR_MOCK_MODE", "true")

	defer func() {
		_ = os.Unsetenv("OTBR_RADIO_URL")
		_ = os.Unsetenv("OTBR_LOG_LEVEL")
		_ = os.Unsetenv("OTBR_AUTO_DISCOVER")
		_ = os.Unsetenv("OTBR_STATE_DIR")
		_ = os.Unsetenv("OTBR_BAUDRATE")
		_ = os.Unsetenv("OTBR_PORT")
		_ = os.Unsetenv("OTBR_MOCK_MODE")
	}()

	cfg := Load()

	if cfg.RadioURL != "spinel+hdlc+uart:///dev/ttyUSB0" {
		t.Errorf("Expected RadioURL 'spinel+hdlc+uart:///dev/ttyUSB0', got %q", cfg.RadioURL)
	}
	if cfg.Baudrate != 115200 {
		t.Errorf("Expected Baudrate 115200, got %d", cfg.Baudrate)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got %q", cfg.LogLevel)
	}
	if cfg.Port != 9090 {
		t.Errorf("Expected Port 9090, got %d", cfg.Port)
	}
	if cfg.AutoDiscover {
		t.Errorf("Expected AutoDiscover false, got %t", cfg.AutoDiscover)
	}
	if cfg.StateDir != "/tmp/state" {
		t.Errorf("Expected StateDir '/tmp/state', got %q", cfg.StateDir)
	}
	if !cfg.MockMode {
		t.Errorf("Expected MockMode true, got %t", cfg.MockMode)
	}
}

func TestGetEnvBoolInvalid(t *testing.T) {
	_ = os.Setenv("OTBR_AUTO_DISCOVER", "not-a-bool")
	defer func() {
		_ = os.Unsetenv("OTBR_AUTO_DISCOVER")
	}()

	cfg := Load()
	// Should fallback to default: true
	if !cfg.AutoDiscover {
		t.Errorf("Expected AutoDiscover fallback to true, got %t", cfg.AutoDiscover)
	}
}

func TestGetEnvIntInvalid(t *testing.T) {
	_ = os.Setenv("OTBR_BAUDRATE", "-100")
	_ = os.Setenv("OTBR_PORT", "invalid")
	defer func() {
		_ = os.Unsetenv("OTBR_BAUDRATE")
		_ = os.Unsetenv("OTBR_PORT")
	}()

	cfg := Load()
	// Should fallback to default: 460800
	if cfg.Baudrate != 460800 {
		t.Errorf("Expected Baudrate default 460800, got %d", cfg.Baudrate)
	}
	// Should fallback to default: 8081
	if cfg.Port != 8081 {
		t.Errorf("Expected Port default 8081, got %d", cfg.Port)
	}
}
