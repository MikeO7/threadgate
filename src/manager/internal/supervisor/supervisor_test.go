package supervisor

import (
	"context"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestNewSupervisor(t *testing.T) {
	s := New("spinel+hdlc+uart:///dev/ttyUSB0", "/data", "info", config.RuntimeModeHardware, "eth0")

	if s.radioURL != "spinel+hdlc+uart:///dev/ttyUSB0" {
		t.Errorf("Expected radioURL spinel+hdlc+uart:///dev/ttyUSB0, got %s", s.radioURL)
	}
	if s.stateDir != "/data" {
		t.Errorf("Expected stateDir /data, got %s", s.stateDir)
	}
	if s.logLevel != "info" {
		t.Errorf("Expected logLevel info, got %s", s.logLevel)
	}
}

func TestSupervisorMock(t *testing.T) {
	s := New("spinel+hdlc+uart:///dev/ttyMOCK0", "/data", "info", config.RuntimeModeMock, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Supervisor.Start in mock mode failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	s.Stop()
}
