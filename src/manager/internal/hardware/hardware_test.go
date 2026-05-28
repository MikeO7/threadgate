package hardware

import (
	"testing"
)

func TestIsKnownHardwareName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"usb-silabs-zbt-1-if00", true},
		{"usb-sonoff_zigbee_3.0_usb_dongle_plus-if00", true},
		{"usb-nordic_semiconductor_nrf52840-if00", true},
		{"usb-ftdi_ft232r-if00", true},
		{"usb-ch340-if00", true},
		{"usb-cp2102-if00", true},
		{"usb-generic_adapter-if00", false},
		{"something_random", false},
	}

	for _, tt := range tests {
		result := isKnownHardwareName(tt.name)
		if result != tt.expected {
			t.Errorf("isKnownHardwareName(%q) = %t; want %t", tt.name, result, tt.expected)
		}
	}
}

func TestTargetSignaturesExist(t *testing.T) {
	// Verify target signatures maps standard coordinators correctly
	if _, ok := targetSignatures["10c4:ea60"]; !ok {
		t.Error("Expected Silicon Labs CP210x CP2102 signature to exist in targetSignatures")
	}
	if _, ok := targetSignatures["1915:528f"]; !ok {
		t.Error("Expected Nordic nRF52840 signature to exist in targetSignatures")
	}
}

func TestDiscoverRadioMock(t *testing.T) {
	path, err := DiscoverRadio(true)
	if err != nil {
		t.Fatalf("DiscoverRadio in mock mode failed: %v", err)
	}
	if path != "/dev/ttyMOCK0" {
		t.Errorf("Expected mock path /dev/ttyMOCK0, got %s", path)
	}
}
