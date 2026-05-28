package radio

import (
	"testing"
)

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
		profile, ok := ParseSpinelURL(tt.url, tt.defaultBaud)
		if ok != tt.expectedOk {
			t.Errorf("ParseSpinelURL(%q) ok = %t, want %t", tt.url, ok, tt.expectedOk)
		}
		if ok {
			if profile.DevicePath != tt.expectedPath {
				t.Errorf("ParseSpinelURL(%q) DevicePath = %q, want %q", tt.url, profile.DevicePath, tt.expectedPath)
			}
			if profile.Baudrate != tt.expectedBaud {
				t.Errorf("ParseSpinelURL(%q) Baudrate = %d, want %d", tt.url, profile.Baudrate, tt.expectedBaud)
			}
			if profile.FlowControl != tt.expectedFlow {
				t.Errorf("ParseSpinelURL(%q) FlowControl = %t, want %t", tt.url, profile.FlowControl, tt.expectedFlow)
			}
		}
	}
}

func TestResolveProfile(t *testing.T) {
	profile, err := ResolveProfile(Config{
		RadioURL: "spinel+hdlc+uart:///dev/ttyUSB9?uart-baudrate=115200",
		Baudrate: 460800,
		MockMode: true,
	}, false)
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	if profile.DevicePath != "/dev/ttyUSB9" || profile.Baudrate != 115200 {
		t.Errorf("unexpected profile: %+v", profile)
	}

	profile, err = ResolveProfile(Config{AutoDiscover: true, Baudrate: 9600, MockMode: true}, false)
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	if profile.DevicePath != "/dev/ttyMOCK0" {
		t.Errorf("unexpected profile: %+v", profile)
	}

	_, err = ResolveProfile(Config{AutoDiscover: false, MockMode: true}, false)
	if err == nil {
		t.Error("expected error when auto-discovery is disabled and URL is empty")
	}
}
