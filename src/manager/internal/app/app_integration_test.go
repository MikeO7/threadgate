package app

import (
	"context"
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
		FlowControl:         false,
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
	port := reserveTCPPort(t)

	cfg := &config.Config{
		Port:         port,
		RadioURL:     "spinel+hdlc+uart:///dev/ttyDOESNOTEXIST999?uart-baudrate=115200",
		AutoDiscover: false,
		Baudrate:     115200,
		Runtime:      config.RuntimeModeHardware,
		StateDir:     t.TempDir(),
		LogLevel:     "info",
	}

	t.Setenv("PATH", t.TempDir())

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
		t.Fatalf("Run should continue despite radio probe failure: %v", err)
	}
}

func TestAppRunMock(t *testing.T) {
	port := reserveTCPPort(t)

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
	port := reserveTCPPort(t)
	ln := holdAppTCPPort(t, port)

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
