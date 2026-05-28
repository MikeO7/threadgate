package app

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/api"
	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/supervisor"
)

func TestFindAvailablePortFallback(t *testing.T) {
	if got := findAvailablePort(65500); got != 65500 {
		t.Errorf("expected fallback port 65500, got %d", got)
	}
}

func TestRadioBindingHardwareProbeError(t *testing.T) {
	cfg := &config.Config{
		Baudrate: 115200,
		Runtime:  config.RuntimeModeHardware,
		RadioURL: "spinel+hdlc+uart:///dev/ttyDOESNOTEXIST999?uart-baudrate=115200",
	}
	tracker := runtime.NewTracker()
	radioBinding, err := radio.NewBinding(cfg, tracker)
	if err != nil {
		t.Fatalf("NewBinding failed: %v", err)
	}
	if tracker.GetStatus().ProbeError == "" {
		t.Fatal("expected probe failure for missing device")
	}
	_ = radioBinding
}

func TestRadioBindingExplicitURL(t *testing.T) {
	cfg := &config.Config{
		RadioURL:            "spinel+hdlc+uart:///dev/ttyUSB1?uart-baudrate=115200",
		AutoDiscover:        false,
		Baudrate:            460800,
		ExplicitFlowControl: true,
		FlowControl:         true,
		Runtime:             config.RuntimeModeHardware,
	}
	radioBinding, err := radio.NewBinding(cfg, runtime.NewTracker())
	if err != nil {
		t.Fatalf("NewBinding failed: %v", err)
	}
	url := radioBinding.CurrentSpinelURL()
	if url != "spinel+hdlc+uart:///dev/ttyUSB1?uart-baudrate=115200&uart-flow-control=0" {
		t.Errorf("unexpected URL: %q", url)
	}
}

func TestRadioBindingConfigError(t *testing.T) {
	cfg := &config.Config{
		AutoDiscover: false,
		Runtime:      config.RuntimeModeHardware,
	}
	_, err := radio.NewBinding(cfg, runtime.NewTracker())
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestAppRunConfigError(t *testing.T) {
	cfg := &config.Config{
		AutoDiscover: false,
		Runtime:      config.RuntimeModeHardware,
		StateDir:     t.TempDir(),
	}
	err := (&App{cfg: cfg}).Run()
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestAppRunWithProbeError(t *testing.T) {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{
		Port:         port,
		RadioURL:     "spinel+hdlc+uart:///dev/ttyDOESNOTEXIST999?uart-baudrate=115200",
		AutoDiscover: false,
		Baudrate:     115200,
		Runtime:      config.RuntimeModeHardware,
		StateDir:     t.TempDir(),
		LogLevel:     "info",
	}

	// Keep hardware runtime but hide system daemons so supervisor boot fails consistently in CI.
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })

	oldWait := waitForShutdownHook
	waitForShutdownHook = func(server *api.Server, super *supervisor.Supervisor, cancel context.CancelFunc, _ <-chan error, _ chan os.Signal) {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		super.Stop()
		cancel()
	}
	t.Cleanup(func() { waitForShutdownHook = oldWait })

	err = (&App{cfg: cfg}).Run()
	if err == nil {
		t.Fatal("expected supervisor boot failure on hardware runtime")
	}
}

func TestAppRunMock(t *testing.T) {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{
		Port:         port,
		Runtime:      config.RuntimeModeMock,
		AutoDiscover: true,
		Baudrate:     460800,
		StateDir:     t.TempDir(),
		LogLevel:     "info",
	}

	oldWait := waitForShutdownHook
	waitForShutdownHook = func(server *api.Server, super *supervisor.Supervisor, cancel context.CancelFunc, _ <-chan error, _ chan os.Signal) {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		super.Stop()
		cancel()
	}
	t.Cleanup(func() { waitForShutdownHook = oldWait })

	if err := (&App{cfg: cfg}).Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestAppRunPortRemap(t *testing.T) {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{
		Port:         port,
		Runtime:      config.RuntimeModeMock,
		AutoDiscover: true,
		Baudrate:     460800,
		StateDir:     t.TempDir(),
	}

	oldWait := waitForShutdownHook
	waitForShutdownHook = func(server *api.Server, super *supervisor.Supervisor, cancel context.CancelFunc, _ <-chan error, _ chan os.Signal) {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		super.Stop()
		cancel()
		_ = ln.Close()
	}
	t.Cleanup(func() { waitForShutdownHook = oldWait })

	if err := (&App{cfg: cfg}).Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func testSupervisor(t *testing.T, cfg *config.Config) *supervisor.Supervisor {
	t.Helper()
	if !cfg.AutoDiscover && cfg.RadioURL == "" {
		cfg.AutoDiscover = true
	}
	runtimeEnv, err := env.Bootstrap(cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return supervisor.New(runtimeEnv, supervisor.ExecLauncher{})
}

func bootstrapAPIServer(t *testing.T, cfg *config.Config) (*api.Server, <-chan error) {
	t.Helper()
	if !cfg.AutoDiscover && cfg.RadioURL == "" {
		cfg.AutoDiscover = true
	}
	runtimeEnv, err := env.Bootstrap(cfg)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	return startAPIServer(runtimeEnv)
}
