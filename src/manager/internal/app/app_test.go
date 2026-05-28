package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

func TestNewApp(t *testing.T) {
	t.Setenv("OTBR_MOCK_MODE", "true")

	application := New()
	if application == nil {
		t.Fatal("Expected New() to return a non-nil App instance")
	}
	if application.cfg == nil {
		t.Fatal("Expected App.cfg to be initialized")
	}
	if !application.cfg.Runtime.IsMock() {
		t.Error("Expected mock runtime when OTBR_MOCK_MODE is enabled")
	}
}

func TestFindAvailablePort(t *testing.T) {
	port := reserveTCPPort(t)

	got := findAvailablePort(port)
	if got != port {
		t.Errorf("expected port %d, got %d", port, got)
	}
}

func TestRadioBindingMock(t *testing.T) {
	cfg := &config.Config{
		AutoDiscover: true,
		Baudrate:     460800,
		Runtime:      config.RuntimeModeMock,
	}
	tracker := runtime.NewTracker()
	radioBinding, err := radio.NewBinding(cfg, tracker)
	if err != nil {
		t.Fatalf("NewBinding failed: %v", err)
	}
	url := radioBinding.CurrentSpinelURL()
	if url != "spinel+hdlc+uart:///dev/ttyMOCK0?uart-baudrate=460800" {
		t.Errorf("unexpected radio URL: %q", url)
	}
	tracker2 := runtime.NewTracker()
	radioBinding, err = radio.NewBinding(cfg, tracker2)
	if err != nil {
		t.Fatal(err)
	}
	if tracker2.GetStatus().ProbedVersion == "" {
		t.Error("expected mock probed version")
	}
	if tracker2.GetStatus().RadioPath != "/dev/ttyMOCK0" {
		t.Errorf("expected /dev/ttyMOCK0, got %q", tracker2.GetStatus().RadioPath)
	}
	_ = radioBinding
}

func TestStartAPIServer(t *testing.T) {
	port := reserveTCPPort(t)

	cfg := &config.Config{
		Port:         port,
		Runtime:      config.RuntimeModeMock,
		AutoDiscover: true,
		StateDir:     t.TempDir(),
	}

	runtimeEnv, err := env.Bootstrap(cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	_, errChan := startAPIServer(runtimeEnv)

	deadline := time.Now().Add(2 * time.Second)
	var dialer net.Dialer
	for time.Now().Before(deadline) {
		conn, dialErr := dialer.DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if dialErr == nil {
			_ = conn.Close()
			return
		}
		select {
		case err := <-errChan:
			if err != nil && err != http.ErrServerClosed {
				t.Fatalf("server failed to start: %v", err)
			}
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Fatal("API server did not become reachable")
}
