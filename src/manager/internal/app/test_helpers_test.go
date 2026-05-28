package app

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

// reserveTCPPort picks a free TCP port on loopback (number only; no listener held).
func reserveTCPPort(t *testing.T) int {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// holdAppTCPPort binds the same address shape the API server uses (all interfaces).
func holdAppTCPPort(t *testing.T, port int) *net.TCPListener {
	t.Helper()
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatalf("hold :%d: %v", port, err)
	}
	return ln.(*net.TCPListener)
}

func waitTCPPortReady(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var dialer net.Dialer
	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("127.0.0.1:%d did not become reachable", port)
}
