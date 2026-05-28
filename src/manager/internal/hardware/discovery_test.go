package hardware

import (
	"os"
	"path/filepath"
	"testing"
)

func withDiscoveryPaths(t *testing.T, root string) {
	t.Helper()
	t.Cleanup(SetDiscoveryPathsForTest(
		filepath.Join(root, "dev"),
		filepath.Join(root, "dev", "serial", "by-id"),
		filepath.Join(root, "sys", "bus", "usb", "devices"),
	))
}

func TestDiscoverBySerialID(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	byID := serialByIDDir
	if err := os.MkdirAll(byID, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(devDir, "ttyUSB0")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	linkName := filepath.Join(byID, "usb-silabs-zbt-1-if00-port0")
	if err := os.Symlink(target, linkName); err != nil {
		t.Fatal(err)
	}

	path, baud, flow, err := discoverBySerialID()
	if err != nil {
		t.Fatalf("discoverBySerialID failed: %v", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if path != resolvedTarget {
		t.Errorf("expected path %q, got %q", resolvedTarget, path)
	}
	if baud != 460800 {
		t.Errorf("expected baud 460800, got %d", baud)
	}
	if !flow {
		t.Error("expected flow control true for ZBT-1")
	}
}

func TestDiscoverBySerialIDMissingDir(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	_, _, _, err := discoverBySerialID()
	if err == nil {
		t.Fatal("expected error when serial by-id dir is missing")
	}
}

func TestDiscoverBySysFS(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	deviceDir := filepath.Join(sysUSBDevicesDir, "1-1")
	if err := os.MkdirAll(deviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "idVendor"), []byte("10c4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "idProduct"), []byte("ea60\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ttyDir := filepath.Join(deviceDir, "ttyUSB0")
	if err := os.MkdirAll(ttyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}

	path, baud, flow, err := discoverBySysFS()
	if err != nil {
		t.Fatalf("discoverBySysFS failed: %v", err)
	}
	expected := filepath.Join(devDir, "ttyUSB0")
	if path != expected {
		t.Errorf("expected path %q, got %q", expected, path)
	}
	if baud != 460800 {
		t.Errorf("expected baud 460800, got %d", baud)
	}
	if !flow {
		t.Error("expected flow control true for CP2102")
	}
}

func TestDiscoverByTTY(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "ttyACM0"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	path, err := discoverByTTY()
	if err != nil {
		t.Fatalf("discoverByTTY failed: %v", err)
	}
	if path != filepath.Join(devDir, "ttyACM0") {
		t.Errorf("unexpected path: %q", path)
	}
}

func TestDiscoverRadioNoDevices(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	_, _, _, err := DiscoverRadio(false)
	if err == nil {
		t.Fatal("expected error when no devices are present")
	}
}

func TestDiscoverRadioViaSerialID(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	byID := serialByIDDir
	if err := os.MkdirAll(byID, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(devDir, "ttyUSB1")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	linkName := filepath.Join(byID, "usb-cp2102-if00-port0")
	if err := os.Symlink(target, linkName); err != nil {
		t.Fatal(err)
	}

	path, _, _, err := DiscoverRadio(false)
	if err != nil {
		t.Fatalf("DiscoverRadio failed: %v", err)
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if path != resolvedTarget {
		t.Errorf("expected %q, got %q", resolvedTarget, path)
	}
}

func TestInspectSysFSDeviceRejectsBadPath(t *testing.T) {
	if _, ok := inspectSysFSDevice("/etc/passwd"); ok {
		t.Fatal("expected path traversal to be rejected")
	}
}

func TestScanSysFSMissingDir(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	devices := scanSysFS()
	if len(devices) != 0 {
		t.Fatalf("expected no devices, got %d", len(devices))
	}
}

func TestFindTTYNode(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	usbPath := filepath.Join(sysUSBDevicesDir, "1-2")
	if err := os.MkdirAll(filepath.Join(usbPath, "ttyACM1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got := findTTYNode(usbPath)
	want := filepath.Join(devDir, "ttyACM1")
	if got != want {
		t.Errorf("findTTYNode = %q, want %q", got, want)
	}
}

func TestFindTTYNodeRejectsBadPath(t *testing.T) {
	if got := findTTYNode("/tmp/not-usb"); got != "" {
		t.Fatalf("expected empty path, got %q", got)
	}
}

func TestDiscoverRadioViaTTY(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "ttyUSB2"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	path, baud, flow, err := DiscoverRadio(false)
	if err != nil {
		t.Fatalf("DiscoverRadio failed: %v", err)
	}
	if path != filepath.Join(devDir, "ttyUSB2") {
		t.Errorf("unexpected path: %q", path)
	}
	if baud != 0 || flow {
		t.Errorf("expected generic tty fallback settings, got baud=%d flow=%t", baud, flow)
	}
}

func TestDiscoverBySerialIDReadError(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)
	if err := os.MkdirAll(serialByIDDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serialByIDDir, "not-a-dir"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := discoverBySerialID()
	if err != nil {
		t.Fatalf("expected graceful empty result, got error: %v", err)
	}
}

func TestDiscoverByTTYReadError(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)
	if err := os.WriteFile(devDir, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := discoverByTTY()
	if err == nil {
		t.Fatal("expected read error for invalid dev dir")
	}
}

func TestInspectSysFSDeviceNoTTY(t *testing.T) {
	root := t.TempDir()
	withDiscoveryPaths(t, root)

	deviceDir := filepath.Join(sysUSBDevicesDir, "2-2")
	if err := os.MkdirAll(deviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "idVendor"), []byte("10c4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deviceDir, "idProduct"), []byte("ea60\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, ok := inspectSysFSDevice(deviceDir); ok {
		t.Fatal("expected no match without tty node")
	}
}
