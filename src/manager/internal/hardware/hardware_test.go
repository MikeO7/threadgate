package hardware

import (
	"strings"
	"testing"
)

const (
	hwSilabsZBT    = "usb-silabs-zbt-1-if00"
	hwNordicNRF528 = "usb-nordic_semiconductor_nrf52840-if00"
	hwFTDI         = "usb-ftdi_ft232r-if00"
	hwCH340        = "usb-ch340-if00"
	hwRandom       = "something_random"
)

func TestIsKnownHardwareName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{hwSilabsZBT, true},
		{"usb-sonoff_zigbee_3.0_usb_dongle_plus-if00", true},
		{hwNordicNRF528, true},
		{hwFTDI, true},
		{hwCH340, true},
		{"usb-cp2102-if00", true},
		{"usb-generic_adapter-if00", false},
		{hwRandom, false},
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
	path, baud, flow, err := DiscoverRadio(true)
	if err != nil {
		t.Fatalf("DiscoverRadio in mock mode failed: %v", err)
	}
	if path != "/dev/ttyMOCK0" {
		t.Errorf("Expected mock path /dev/ttyMOCK0, got %s", path)
	}
	if baud != 460800 {
		t.Errorf("Expected mock baudrate 460800, got %d", baud)
	}
	if flow {
		t.Errorf("Expected mock flow control false, got %t", flow)
	}
}

func TestGetBaudrateFromHardwareName(t *testing.T) {
	tests := []struct {
		name     string
		expected int
	}{
		{"usb-silabs-zbt-1-if00", 460800},
		{"usb-sonoff_zigbee_3.0_usb_dongle_plus-if00", 460800},
		{"usb-nordic_semiconductor_nrf52840-if00", 115200},
		{"usb-nordic_semiconductor_openthread-if00", 115200},
		{"usb-ftdi_ft232r-if00", 115200},
		{"usb-ch340-if00", 115200},
		{"usb-cp2102-if00", 460800},
		{"usb-prolific_pl2303-if00", 115200},
		{"usb-sonoff_zbdongle-e-if00", 460800},
		{"usb-generic_adapter-if00", 0},
		{"something_random", 0},
	}

	for _, tt := range tests {
		result := getBaudrateFromHardwareName(tt.name)
		if result != tt.expected {
			t.Errorf("getBaudrateFromHardwareName(%q) = %d; want %d", tt.name, result, tt.expected)
		}
	}
}

func TestDetectMacSerialSignature(t *testing.T) {
	oldRun := runIoregCmd
	defer func() { runIoregCmd = oldRun }()

	// 1. Mocking a CP2102N device
	runIoregCmd = func() ([]byte, error) {
		return []byte(`
      +-o SONOFF Dongle Plus MG24@01200000  <class IOUSBHostDevice, id 0x1000891d0>
          {
            "kUSBProductString" = "SONOFF Dongle Plus MG24"
            "USB Vendor Name" = "SONOFF"
            "idVendor" = 4292
            "idProduct" = 60000
          }
`), nil
	}

	desc, vid, pid, found := DetectMacSerialSignature()
	if !found {
		t.Fatal("expected device to be detected")
	}
	if !strings.Contains(desc, "Silicon Labs CP210x") && !strings.Contains(desc, "SONOFF") {
		t.Errorf("unexpected description: %q", desc)
	}
	if vid != "10c4" || pid != "ea60" {
		t.Errorf("unexpected VID/PID: %s:%s", vid, pid)
	}

	// 2. Mocking a generic Nordic device
	runIoregCmd = func() ([]byte, error) {
		return []byte(`
      +-o Nordic nRF52840@01200000  <class IOUSBHostDevice, id 0x1000891d0>
          {
            "kUSBProductString" = "Nordic Semiconductor nRF52840 Thread Dongle (RCP)"
            "USB Vendor Name" = "Nordic"
            "idVendor" = 6421
            "idProduct" = 21135
          }
`), nil
	}

	_, vid, pid, found = DetectMacSerialSignature()
	if !found {
		t.Fatal("expected Nordic device to be detected")
	}
	if vid != "1915" || pid != "528f" {
		t.Errorf("unexpected Nordic VID/PID: %s:%s", vid, pid)
	}
}
