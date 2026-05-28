package supervisor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

func writeFakeCommands(t *testing.T, commands map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range commands {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type fakeLauncher struct {
	binDir string
}

func (f fakeLauncher) CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, filepath.Join(f.binDir, name), arg...)
}

func mockConfig(autoDiscover bool) *config.Config {
	cfg := &config.Config{
		AutoDiscover: autoDiscover,
		Baudrate:     460800,
		Runtime:      config.RuntimeModeMock,
		LogLevel:     "info",
		StateDir:     "/data",
	}
	if !autoDiscover {
		cfg.RadioURL = "spinel+hdlc+uart:///dev/ttyMOCK0"
	}
	return cfg
}

func newTestSupervisor(t *testing.T, cfg *config.Config, status *runtime.Tracker, launcher ProcessLauncher) *Supervisor {
	t.Helper()
	if cfg == nil {
		cfg = mockConfig(false)
	}
	if status == nil {
		status = runtime.NewTracker()
	}
	runtimeEnv, err := env.BootstrapWithStatus(cfg, status)
	if err != nil {
		t.Fatalf("BootstrapWithStatus: %v", err)
	}
	return New(runtimeEnv, launcher)
}

func TestSupervisorMock(t *testing.T) {
	cfg := mockConfig(false)
	cfg.RadioURL = "spinel+hdlc+uart:///dev/ttyMOCK0"
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Supervisor.Start in mock mode failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	s.Stop()
}

func TestSupervisorStartWithFakeDaemons(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		"dbus-daemon":  "#!/bin/sh\nwhile true; do sleep 0.2; done\n",
		"avahi-daemon": "#!/bin/sh\nwhile true; do sleep 0.2; done\n",
		"otbr-agent":   "#!/bin/sh\nexit 0\n",
	})

	cfg := mockConfig(false)
	cfg.Runtime = config.RuntimeModeHardware
	s := newTestSupervisor(t, cfg, runtime.NewTracker(), fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(1500 * time.Millisecond)
	s.Stop()
}

func TestRunAgentOnce(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		"otbr-agent": "#!/bin/sh\nexit 0\n",
	})
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	cfg := mockConfig(false)
	cfg.RadioURL = "spinel+hdlc+uart:///dev/ttyMOCK0"
	cfg.BackboneIF = "eth0"
	s := newTestSupervisor(t, cfg, runtime.NewTracker(), fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		s.runAgentOnce()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runAgentOnce did not complete")
	}
	cancel()
}

func TestRunAgentOnceAutoDiscover(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		"otbr-agent": "#!/bin/sh\nexit 0\n",
	})
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	tracker := runtime.NewTracker()
	cfg := mockConfig(true)
	s := newTestSupervisor(t, cfg, tracker, fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	s.runAgentOnce()
	cancel()

	if tracker.GetStatus().RadioPath != "/dev/ttyMOCK0" {
		t.Errorf("expected radio path update, got %q", tracker.GetStatus().RadioPath)
	}
	if tracker.GetStatus().ProbedVersion == "" {
		t.Error("expected probed version after refresh")
	}
}

func TestRunMockAgentLoopInnerCancel(t *testing.T) {
	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		s.runMockAgentLoop()
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runMockAgentLoop did not exit on inner cancel")
	}
}

func TestRunMockAgentLoopTimeoutCycle(t *testing.T) {
	oldSleep := mockAgentSleep
	mockAgentSleep = 20 * time.Millisecond
	t.Cleanup(func() { mockAgentSleep = oldSleep })

	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		s.runMockAgentLoop()
		close(done)
	}()

	time.Sleep(60 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runMockAgentLoop did not exit")
	}
}

func TestRunMockAgentLoopCancel(t *testing.T) {
	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		s.runMockAgentLoop()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runMockAgentLoop did not exit after cancel")
	}
}

func TestRunAgentOnceStartFailure(t *testing.T) {
	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	done := make(chan struct{})
	go func() {
		s.runAgentOnce()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("runAgentOnce did not return after missing binary retry delay")
	}
	cancel()
}

func TestSupervisorStartDBusFailure(t *testing.T) {
	cfg := mockConfig(false)
	cfg.RadioURL = "spinel+hdlc+uart:///dev/ttyUSB0"
	cfg.Runtime = config.RuntimeModeHardware
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: "/definitely/missing"})
	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("expected dbus start failure")
	}
}

func TestStartAvahi(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		"avahi-daemon": "#!/bin/sh\nwhile true; do sleep 0.2; done\n",
	})

	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)
	s.startAvahi()
	cancel()
}

func TestStartAvahiMissingBinary(t *testing.T) {
	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: "/missing"})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)
	s.startAvahi()
	cancel()
}

func TestRunAgentLoop(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		"otbr-agent": "#!/bin/sh\nexit 0\n",
	})
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	go s.runAgentLoop()
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestRunAgentOnceDiscoveryFailure(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		"otbr-agent": "#!/bin/sh\nexit 0\n",
	})
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	root := t.TempDir()
	restore := hardware.SetDiscoveryPathsForTest(
		filepath.Join(root, "dev"),
		filepath.Join(root, "serial", "by-id"),
		filepath.Join(root, "sys", "bus", "usb", "devices"),
	)
	t.Cleanup(restore)

	cfg := mockConfig(true)
	cfg.RadioURL = "spinel+hdlc+uart:///dev/ttyFALLBACK0"
	cfg.Runtime = config.RuntimeModeHardware
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)
	s.runAgentOnce()
	cancel()
}
