package hardware

import (
	"path/filepath"
	"testing"
)

const (
	sonoffMG24ByIDName = "usb-SONOFF_SONOFF_Dongle_Plus_MG24_52e956dd50a3ef11975b4ebd61ce3355-if00-port0"
	sonoffMG24Desc     = "SONOFF Dongle Plus MG24"
)

func TestDetectUSBSerialSignatureViaSysFS(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	usbID := "1-1"
	setupSysFSUSBDevice(t, usbID, "10c4", "ea60", linuxTTYUSB0)
	setupSerialByIDLink(t, sonoffMG24ByIDName, linuxTTYUSB0)

	desc, vid, pid, found := DetectUSBSerialSignature(filepath.Join(root, "dev", linuxTTYUSB0))
	if !found {
		t.Fatal("expected USB serial signature to be detected via sysfs")
	}
	if vid != "10c4" || pid != "ea60" {
		t.Fatalf("unexpected vid:pid %s:%s", vid, pid)
	}
	if desc != sonoffMG24Desc {
		t.Fatalf("unexpected description %q", desc)
	}
}

func TestDetectUSBSerialSignatureViaSerialByIDOnly(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	devDir := filepath.Join(root, "dev")
	setupSerialByIDLink(t, sonoffMG24ByIDName, linuxTTYUSB0)

	desc, vid, pid, found := DetectUSBSerialSignature(filepath.Join(devDir, linuxTTYUSB0))
	if !found {
		t.Fatal("expected USB serial signature from by-id name")
	}
	if desc != sonoffMG24Desc {
		t.Fatalf("unexpected description %q", desc)
	}
	if vid != "" || pid != "" {
		t.Fatalf("expected empty vid/pid without sysfs, got %s:%s", vid, pid)
	}
}

func TestParseSerialByIDProductName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{
			in:   sonoffMG24ByIDName,
			want: sonoffMG24Desc,
		},
		{
			in:   "usb-ITead_Sonoff_ZBDongle-E-if00-port0",
			want: "ITead Sonoff ZBDongle-E",
		},
	}
	for _, tc := range tests {
		got := parseSerialByIDProductName(tc.in)
		if got != tc.want {
			t.Errorf("parseSerialByIDProductName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
