package radio

import (
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

func TestBindingMockRefresh(t *testing.T) {
	tracker := runtime.NewTracker()
	cfg := &config.Config{
		AutoDiscover: true,
		Baudrate:     460800,
		Runtime:      config.RuntimeModeMock,
	}
	b, err := NewBinding(cfg, tracker)
	if err != nil {
		t.Fatalf("NewBinding: %v", err)
	}
	if b.CurrentSpinelURL() != "spinel+hdlc+uart:///dev/ttyMOCK0?uart-baudrate=460800" {
		t.Errorf("unexpected URL: %q", b.CurrentSpinelURL())
	}
	if err := b.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	status := tracker.GetStatus()
	if status.RadioPath != "/dev/ttyMOCK0" {
		t.Errorf("expected radio path /dev/ttyMOCK0, got %q", status.RadioPath)
	}
	if status.ProbedVersion == "" {
		t.Error("expected mock probed version after refresh")
	}
}

func TestParseSpinelURL(t *testing.T) {
	tests := []struct {
		url          string
		defaultBaud  int
		expectedPath string
		expectedBaud int
		expectedFlow bool
		expectedOk   bool
	}{
		{
			url:          "spinel+hdlc+uart:///dev/ttyUSB0?uart-baudrate=460800&uart-flow-control=1",
			defaultBaud:  115200,
			expectedPath: "/dev/ttyUSB0",
			expectedBaud: 460800,
			expectedFlow: true,
			expectedOk:   true,
		},
		{
			url:          "spinel+hdlc+uart:///dev/ttyACM1",
			defaultBaud:  115200,
			expectedPath: "/dev/ttyACM1",
			expectedBaud: 115200,
			expectedFlow: false,
			expectedOk:   true,
		},
		{
			url:          "udp://[::1]:12345",
			defaultBaud:  115200,
			expectedPath: "",
			expectedBaud: 0,
			expectedFlow: false,
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			p, ok := parseSpinelURL(tt.url, tt.defaultBaud)
			assertParseSpinelURL(t, tt, p, ok)
		})
	}
}

func assertParseSpinelURL(t *testing.T, tt struct {
	url          string
	defaultBaud  int
	expectedPath string
	expectedBaud int
	expectedFlow bool
	expectedOk   bool
}, p profile, ok bool) {
	t.Helper()
	if ok != tt.expectedOk {
		t.Errorf("parseSpinelURL(%q) ok = %t, want %t", tt.url, ok, tt.expectedOk)
		return
	}
	if !ok {
		return
	}
	if p.DevicePath != tt.expectedPath {
		t.Errorf("parseSpinelURL(%q) DevicePath = %q, want %q", tt.url, p.DevicePath, tt.expectedPath)
	}
	if p.Baudrate != tt.expectedBaud {
		t.Errorf("parseSpinelURL(%q) Baudrate = %d, want %d", tt.url, p.Baudrate, tt.expectedBaud)
	}
	if p.FlowControl != tt.expectedFlow {
		t.Errorf("parseSpinelURL(%q) FlowControl = %t, want %t", tt.url, p.FlowControl, tt.expectedFlow)
	}
}

func TestResolveProfile(t *testing.T) {
	p, err := resolveProfile(radioConfig{
		RadioURL: "spinel+hdlc+uart:///dev/ttyUSB9?uart-baudrate=115200",
		Baudrate: 460800,
		MockMode: true,
	}, false)
	if err != nil {
		t.Fatalf("resolveProfile failed: %v", err)
	}
	if p.DevicePath != "/dev/ttyUSB9" || p.Baudrate != 115200 {
		t.Errorf("unexpected profile: %+v", p)
	}

	p, err = resolveProfile(radioConfig{AutoDiscover: true, Baudrate: 9600, MockMode: true}, false)
	if err != nil {
		t.Fatalf("resolveProfile failed: %v", err)
	}
	if p.DevicePath != "/dev/ttyMOCK0" {
		t.Errorf("unexpected profile: %+v", p)
	}

	_, err = resolveProfile(radioConfig{AutoDiscover: false, MockMode: true}, false)
	if err == nil {
		t.Error("expected error when auto-discovery is disabled and URL is empty")
	}
}
