package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestWaitForShutdownSignal(t *testing.T) {
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{
		Port:     port,
		Runtime:  config.RuntimeModeMock,
		StateDir: t.TempDir(),
	}

	server, _ := bootstrapAPIServer(t, cfg)
	deadline := time.Now().Add(2 * time.Second)
	var dialer net.Dialer
	for time.Now().Before(deadline) {
		conn, dialErr := dialer.DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if dialErr == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

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
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{Port: port, Runtime: config.RuntimeModeMock, StateDir: t.TempDir()}
	server, _ := bootstrapAPIServer(t, cfg)
	time.Sleep(50 * time.Millisecond)

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
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := &config.Config{Port: port, Runtime: config.RuntimeModeMock, StateDir: t.TempDir()}
	server, _ := bootstrapAPIServer(t, cfg)
	time.Sleep(50 * time.Millisecond)

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
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	got := findAvailablePort(port)
	if got == port {
		if got < port || got >= port+100 {
			t.Fatalf("expected port in range [%d,%d), got %d", port, port+100, got)
		}
	}
}
