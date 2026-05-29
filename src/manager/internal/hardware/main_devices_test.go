package hardware

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	silabsVID    = "10c4"
	silabsPID    = "ea60"
	linuxTTYUSB0 = "ttyUSB0"
	linuxTTYACM0 = "ttyACM0"
)

// mainCoordinator represents a popular Thread USB coordinator for discovery tests.
type mainCoordinator struct {
	name           string
	vid            string
	pid            string
	serialByIDName string
	wantBaud       int
	wantFlow       bool
}

var mainCoordinators = []mainCoordinator{
	{
		name:           "Home Assistant Connect ZBT-1",
		vid:            silabsVID,
		pid:            silabsPID,
		serialByIDName: "usb-Silicon_Labs_Home_Assistant_Connect_ZBT-1-if00-port0",
		wantBaud:       460800,
		wantFlow:       true,
	},
	{
		name:           "Nabu Casa SkyConnect",
		vid:            silabsVID,
		pid:            silabsPID,
		serialByIDName: "usb-Nabu_Casa_HA_SkyConnect-if00-port0",
		wantBaud:       460800,
		wantFlow:       true,
	},
	{
		name:           "Sonoff ZBDongle-E",
		vid:            silabsVID,
		pid:            silabsPID,
		serialByIDName: "usb-ITead_Sonoff_ZBDongle-E-if00-port0",
		wantBaud:       460800,
		wantFlow:       true,
	},
	{
		name:           "Sonoff Dongle Plus MG24",
		vid:            silabsVID,
		pid:            silabsPID,
		serialByIDName: "usb-SONOFF_Dongle_Plus_MG24-if00-port0",
		wantBaud:       460800,
		wantFlow:       true,
	},
	{
		name:           "Nordic nRF52840 Thread Dongle",
		vid:            "1915",
		pid:            "528f",
		serialByIDName: "usb-Nordic_Semiconductor_nRF52840_Dongle-if00-port0",
		wantBaud:       115200,
		wantFlow:       false,
	},
	{
		name:           "Nordic Custom Thread Coordinator",
		vid:            "1915",
		pid:            "cafe",
		serialByIDName: "usb-Nordic_Semiconductor_OpenThread-if00-port0",
		wantBaud:       115200,
		wantFlow:       false,
	},
	{
		name:           "WCH CH340",
		vid:            "1a86",
		pid:            "7523",
		serialByIDName: "usb-1a86_USB_Serial-ch340-if00-port0",
		wantBaud:       115200,
		wantFlow:       false,
	},
	{
		name:           "WCH CH341",
		vid:            "1a86",
		pid:            "5523",
		serialByIDName: "usb-1a86_USB_Serial-ch341-if00-port0",
		wantBaud:       115200,
		wantFlow:       false,
	},
	{
		name:           "FTDI FT232",
		vid:            "0403",
		pid:            "6001",
		serialByIDName: "usb-FTDI_FT232R_USB_UART-if00-port0",
		wantBaud:       115200,
		wantFlow:       false,
	},
	{
		name:           "Prolific PL2303",
		vid:            "067b",
		pid:            "2303",
		serialByIDName: "usb-Prolific_Technology_Inc._USB-Serial_Controller-if00-port0",
		wantBaud:       115200,
		wantFlow:       false,
	},
}

func setupSysFSUSBDevice(t *testing.T, usbID, vid, pid, ttyLeaf string) {
	t.Helper()
	deviceDir := filepath.Join(sysUSBDevicesDir, usbID)
	if err := os.MkdirAll(filepath.Join(deviceDir, ttyLeaf), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "idVendor"), []byte(vid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "idProduct"), []byte(pid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(devDir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func setupSerialByIDLink(t *testing.T, linkName, ttyName string) string {
	t.Helper()
	if err := os.MkdirAll(serialByIDDir, 0o750); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(devDir, ttyName)
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(serialByIDDir, linkName)
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func assertDiscoverRadio(t *testing.T, wantPath string, wantBaud int, wantFlow bool) {
	t.Helper()
	path, baud, flow, err := DiscoverRadio(false)
	if err != nil {
		t.Fatalf("DiscoverRadio failed: %v", err)
	}
	if path != wantPath {
		t.Errorf("path = %q, want %q", path, wantPath)
	}
	if baud != wantBaud {
		t.Errorf("baud = %d, want %d", baud, wantBaud)
	}
	if flow != wantFlow {
		t.Errorf("flow = %t, want %t", flow, wantFlow)
	}
}

func assertInspectSysFSDevice(t *testing.T, usbID string, sig struct {
	Desc        string
	Baudrate    int
	FlowControl bool
}) {
	t.Helper()
	dev, matched := inspectSysFSDevice(filepath.Join(sysUSBDevicesDir, usbID))
	if !matched {
		t.Fatal("inspectSysFSDevice did not match signature")
	}
	if dev.Baudrate != sig.Baudrate {
		t.Errorf("baudrate = %d, want %d", dev.Baudrate, sig.Baudrate)
	}
	if dev.FlowControl != sig.FlowControl {
		t.Errorf("flow control = %t, want %t", dev.FlowControl, sig.FlowControl)
	}
	if dev.Description != sig.Desc {
		t.Errorf("description = %q, want %q", dev.Description, sig.Desc)
	}
}

func TestDiscoverAllTargetSignaturesViaSysFS(t *testing.T) {
	for key, sig := range targetSignatures {
		t.Run(key, func(t *testing.T) {
			vid, pid, ok := strings.Cut(key, ":")
			if !ok {
				t.Fatalf("invalid signature key %q", key)
			}

			root := t.TempDir()
			withDiscoveryPaths(t, root)
			usbID := "sig-" + key
			setupSysFSUSBDevice(t, usbID, vid, pid, "ttyUSB0")
			assertInspectSysFSDevice(t, usbID, sig)
			assertDiscoverRadio(t, filepath.Join(devDir, "ttyUSB0"), sig.Baudrate, sig.FlowControl)
		})
	}
}

func TestDiscoverAllMainCoordinatorsViaSerialByID(t *testing.T) {
	for _, dev := range mainCoordinators {
		t.Run(dev.name, func(t *testing.T) {
			root := t.TempDir()
			withDiscoveryPaths(t, root)
			wantPath := setupSerialByIDLink(t, dev.serialByIDName, "ttyUSB-coordinator")
			assertDiscoverRadio(t, wantPath, dev.wantBaud, dev.wantFlow)
		})
	}
}

func setupExtraTTYNodes(t *testing.T, nodes []string, skipNode string) {
	t.Helper()
	if err := os.MkdirAll(devDir, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node == skipNode {
			continue
		}
		if err := os.WriteFile(filepath.Join(devDir, node), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiscoverMainCoordinatorsViaTTY(t *testing.T) {
	cases := []struct {
		name     string
		usbID    string
		vid      string
		pid      string
		nodes    []string
		wantNode string
		wantBaud int
		wantFlow bool
	}{
		{
			name: "Sonoff MG24 ttyUSB", usbID: "1-1", vid: silabsVID, pid: silabsPID,
			nodes: []string{linuxTTYUSB0}, wantNode: linuxTTYUSB0, wantBaud: 460800, wantFlow: true,
		},
		{
			name: "Sonoff MG24 prefers ttyUSB over ttyACM", usbID: "1-2", vid: silabsVID, pid: silabsPID,
			nodes: []string{linuxTTYACM0, linuxTTYUSB0}, wantNode: linuxTTYUSB0, wantBaud: 460800, wantFlow: true,
		},
		{
			name: "Nordic nRF52840 ttyACM fallback", usbID: "2-1", vid: "1915", pid: "528f",
			nodes: []string{linuxTTYACM0}, wantNode: linuxTTYACM0, wantBaud: 115200, wantFlow: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			withDiscoveryPaths(t, root)
			setupSysFSUSBDevice(t, tc.usbID, tc.vid, tc.pid, tc.wantNode)
			setupExtraTTYNodes(t, tc.nodes, tc.wantNode)
			assertDiscoverRadio(t, filepath.Join(devDir, tc.wantNode), tc.wantBaud, tc.wantFlow)
		})
	}
}

func TestDetectUSBSerialSignatureAllMainCoordinators(t *testing.T) {
	for _, dev := range mainCoordinators {
		t.Run(dev.name, func(t *testing.T) {
			root := t.TempDir()
			withDiscoveryPaths(t, root)
			usbID := strings.ReplaceAll(strings.ToLower(dev.name), " ", "-")
			setupSysFSUSBDevice(t, usbID, dev.vid, dev.pid, linuxTTYUSB0)
			setupSerialByIDLink(t, dev.serialByIDName, linuxTTYUSB0)

			desc, vid, pid, found := DetectUSBSerialSignature(filepath.Join(root, "dev", linuxTTYUSB0))
			if !found {
				t.Fatal("expected device to be detected via Linux discovery")
			}
			if vid != strings.ToLower(dev.vid) || pid != strings.ToLower(dev.pid) {
				t.Errorf("vid:pid = %s:%s, want %s:%s", vid, pid, dev.vid, dev.pid)
			}
			if desc == "" {
				t.Error("expected non-empty description")
			}
		})
	}
}

func TestIsGenericSerialTTY(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{linuxTTYUSB0, true},
		{linuxTTYACM0, true},
		{"random", false},
	}
	for _, tt := range tests {
		if got := isGenericSerialTTY(tt.name); got != tt.want {
			t.Errorf("isGenericSerialTTY(%q) = %t, want %t", tt.name, got, tt.want)
		}
	}
}

func TestPreferSerialDevice(t *testing.T) {
	paths := []string{
		"/dev/ttyACM0",
		"/dev/ttyUSB0",
	}
	got := preferSerialDevice(paths)
	want := "/dev/ttyUSB0"
	if got != want {
		t.Errorf("preferSerialDevice = %q, want %q", got, want)
	}
}

func TestMainCoordinatorsCoverAllTargetSignatureFamilies(t *testing.T) {
	seen := make(map[string]bool)
	for _, dev := range mainCoordinators {
		key := strings.ToLower(dev.vid + ":" + dev.pid)
		seen[key] = true
		if _, ok := targetSignatures[key]; !ok {
			t.Errorf("main coordinator %q references unknown signature %s", dev.name, key)
		}
	}

	requiredFamilies := []string{
		silabsVID + ":" + silabsPID, "1915:528f", "1915:cafe",
		"1a86:7523", "1a86:5523", "0403:6001", "067b:2303",
	}
	for _, key := range requiredFamilies {
		if !seen[key] {
			t.Errorf("missing main coordinator fixture for signature family %s", key)
		}
	}
}

func TestTargetSignatureCount(t *testing.T) {
	if got := len(targetSignatures); got < 13 {
		t.Errorf("expected at least 13 target signatures, got %d", got)
	}
}

func TestHexVIDPIDConversion(t *testing.T) {
	for key := range targetSignatures {
		vidHex, pidHex, ok := strings.Cut(key, ":")
		if !ok {
			t.Fatalf("bad key %q", key)
		}
		vidDec, err := strconv.ParseInt(vidHex, 16, 64)
		if err != nil {
			t.Fatalf("parse vid %q: %v", vidHex, err)
		}
		pidDec, err := strconv.ParseInt(pidHex, 16, 64)
		if err != nil {
			t.Fatalf("parse pid %q: %v", pidHex, err)
		}
		if fmt.Sprintf("%04x", vidDec) != vidHex {
			t.Errorf("vid round-trip failed for %s", key)
		}
		if fmt.Sprintf("%04x", pidDec) != pidHex {
			t.Errorf("pid round-trip failed for %s", key)
		}
	}
}
