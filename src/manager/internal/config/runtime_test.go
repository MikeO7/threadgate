package config

import (
	"errors"
	"net"
	"os"
	"testing"
)

func TestRuntimeModeIsMock(t *testing.T) {
	if RuntimeModeFromMock(true).IsMock() != true {
		t.Error("expected mock mode")
	}
	if RuntimeModeFromMock(false).IsMock() != false {
		t.Error("expected hardware mode")
	}
}

func TestDetectBackboneInterface(t *testing.T) {
	iface := detectBackboneInterface()
	if iface == "" {
		t.Fatal("expected a backbone interface name")
	}
}

func TestDetectBackboneInterfaceSkipsVirtual(t *testing.T) {
	old := listInterfaces
	listInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{Name: "lo", Flags: net.FlagLoopback | net.FlagUp},
			{Name: "docker0", Flags: net.FlagUp},
			{Name: "wpan0", Flags: net.FlagUp},
			{Name: "eth99", Flags: net.FlagUp},
		}, nil
	}
	t.Cleanup(func() { listInterfaces = old })

	if got := detectBackboneInterface(); got != "eth99" {
		t.Fatalf("expected eth99, got %q", got)
	}
}

func TestDetectBackboneInterfaceListError(t *testing.T) {
	old := listInterfaces
	listInterfaces = func() ([]net.Interface, error) {
		return nil, errors.New("no interfaces")
	}
	t.Cleanup(func() { listInterfaces = old })

	if got := detectBackboneInterface(); got != defaultBackboneInterface {
		t.Fatalf("expected %s fallback, got %q", defaultBackboneInterface, got)
	}
}

func TestDetectBackboneInterfaceDefaultFallback(t *testing.T) {
	old := listInterfaces
	listInterfaces = func() ([]net.Interface, error) {
		return []net.Interface{
			{Name: "lo", Flags: net.FlagLoopback | net.FlagUp},
			{Name: "docker0", Flags: net.FlagUp},
		}, nil
	}
	t.Cleanup(func() { listInterfaces = old })

	if got := detectBackboneInterface(); got != defaultBackboneInterface {
		t.Fatalf("expected %s fallback, got %q", defaultBackboneInterface, got)
	}
}

func TestLoadBackboneFromEnv(t *testing.T) {
	_ = os.Setenv("OTBR_BACKBONE_IF", "wlan-test0")
	t.Cleanup(func() { _ = os.Unsetenv("OTBR_BACKBONE_IF") })

	cfg := Load()
	if cfg.BackboneIF != "wlan-test0" {
		t.Errorf("expected wlan-test0, got %q", cfg.BackboneIF)
	}
}

func TestLoadFlowControlExplicit(t *testing.T) {
	_ = os.Setenv("OTBR_FLOW_CONTROL", "true")
	t.Cleanup(func() { _ = os.Unsetenv("OTBR_FLOW_CONTROL") })

	cfg := Load()
	if !cfg.FlowControl {
		t.Error("expected flow control enabled")
	}
	if !cfg.ExplicitFlowControl {
		t.Error("expected explicit flow control flag")
	}
}
