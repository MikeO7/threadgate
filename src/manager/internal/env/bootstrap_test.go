package env

import (
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestBootstrapMockMode(t *testing.T) {
	cfg := &config.Config{
		Runtime:       config.RuntimeModeFromMock(true),
		StateDir:      "/tmp/threadgate-test",
		RadioURL:      "spinel+hdlc+uart:///dev/ttyUSB0",
		AutoDiscover:  false,
	}
	e, err := Bootstrap(cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if !e.IsMock() {
		t.Fatal("expected mock mode")
	}
	if e.Thread == nil || e.Status == nil {
		t.Fatal("expected wired thread client and status tracker")
	}
}

func TestBootstrapHardwareMode(t *testing.T) {
	cfg := &config.Config{
		Runtime:      config.RuntimeModeHardware,
		StateDir:     "/tmp/threadgate-test",
		RadioURL:     "spinel+hdlc+uart:///dev/ttyUSB0",
		AutoDiscover: false,
	}
	e, err := Bootstrap(cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if e.IsMock() {
		t.Fatal("expected hardware mode")
	}
}
