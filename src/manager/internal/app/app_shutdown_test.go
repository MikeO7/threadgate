package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestWaitForShutdownSignal(t *testing.T) {
	port := reserveTCPPort(t)

	cfg := &config.Config{
		Port:     port,
		Runtime:  config.RuntimeModeMock,
		StateDir: t.TempDir(),
	}

	server, _ := bootstrapAPIServer(t, cfg)
	waitTCPPortReady(t, port)

	super := testSupervisor(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	if err := super.Start(ctx); err != nil {
		t.Fatalf("supervisor start failed: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	sigCh <- syscall.SIGTERM
	waitForShutdown(server, super, cancel, nil, sigCh)
}

func TestWaitForShutdownServerClosed(t *testing.T) {
	port := reserveTCPPort(t)

	cfg := &config.Config{Port: port, Runtime: config.RuntimeModeMock, StateDir: t.TempDir()}
	server, _ := bootstrapAPIServer(t, cfg)
	waitTCPPortReady(t, port)

	super := testSupervisor(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	if err := super.Start(ctx); err != nil {
		t.Fatal(err)
	}

	errChan := make(chan error, 1)
	errChan <- http.ErrServerClosed
	waitForShutdown(server, super, cancel, errChan, nil)
}

func TestWaitForShutdownAPIFailure(t *testing.T) {
	port := reserveTCPPort(t)

	cfg := &config.Config{Port: port, Runtime: config.RuntimeModeMock, StateDir: t.TempDir()}
	server, _ := bootstrapAPIServer(t, cfg)
	waitTCPPortReady(t, port)

	super := testSupervisor(t, cfg)
	ctx, cancel := context.WithCancel(context.Background())
	_ = super.Start(ctx)

	oldFatal := fatalLog
	fatalCalled := false
	fatalLog = func(string, ...any) { fatalCalled = true }
	t.Cleanup(func() { fatalLog = oldFatal })

	errChan := make(chan error, 1)
	errChan <- fmt.Errorf("listener crashed")
	waitForShutdown(server, super, cancel, errChan, nil)
	if !fatalCalled {
		t.Fatal("expected fatal log on critical API failure")
	}
}

func TestFindAvailablePortInUse(t *testing.T) {
	port := reserveTCPPort(t)
	ln := holdAppTCPPort(t, port)
	defer func() { _ = ln.Close() }()

	got := findAvailablePort(port)
	if got == port {
		t.Fatalf("expected a different port when :%d is in use, got same port", port)
	}
	if got < port || got >= port+100 {
		t.Fatalf("expected port in range [%d,%d), got %d", port, port+100, got)
	}
}
