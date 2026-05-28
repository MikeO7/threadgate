// Package hardware implements automatic discovery and detection of USB Thread radio coordinator hardware.
package hardware

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Device represents a discovered USB serial coordinator
type Device struct {
	Path        string
	VendorID    string
	ProductID   string
	Description string
}

// Known Thread coordinator signatures (VID:PID)
var targetSignatures = map[string]struct {
	Desc string
}{
	// Silicon Labs (CP2102, CP2104, CP2102N, J-Link)
	"10c4:ea60": {Desc: "Silicon Labs CP210x USB to UART Bridge (Home Assistant Connect ZBT-1 / Sonoff / SkyConnect)"},
	"10c4:ea70": {Desc: "Silicon Labs CP2105 Dual USB to UART Bridge"},
	"10c4:ea80": {Desc: "Silicon Labs CP2108 Quad USB to UART Bridge"},
	"1366:0101": {Desc: "Silicon Labs Segger J-Link SDK Coordinator"},
	"1366:1015": {Desc: "Silicon Labs J-Link OB Development Link"},

	// Nordic Semiconductor (nRF52840, nRF52833)
	"1915:528f": {Desc: "Nordic Semiconductor nRF52840 Thread Dongle (RCP)"},
	"1915:cafe": {Desc: "Nordic Semiconductor Custom Thread/Zigbee Coordinator"},

	// WCH (CH340, CH341, CH343 - extremely common in DIY/low-cost dongles)
	"1a86:7523": {Desc: "WCH CH340 USB to Serial Coordinator"},
	"1a86:5523": {Desc: "WCH CH341 USB to Serial Coordinator"},
	"1a86:55d2": {Desc: "WCH CH343 High-Speed USB to Serial Coordinator"},

	// FTDI (FT232R, FT231X - DIY/commercial adapters)
	"0403:6001": {Desc: "FTDI FT232 USB to UART Coordinator"},
	"0403:6015": {Desc: "FTDI FT231X USB to UART Coordinator"},

	// Prolific (PL2303)
	"067b:2303": {Desc: "Prolific PL2303 USB to Serial Coordinator"},
}

// DiscoverRadio looks for connected Thread RCP dongles.
// When mockMode is true, returns a simulated device path without scanning hardware.
func DiscoverRadio(mockMode bool) (string, error) {
	log.Println("[Hardware] Running automatic USB serial device discovery...")

	if mockMode {
		log.Println("[Hardware] Mock mode active: returning simulated hardware path /dev/ttyMOCK0")
		return "/dev/ttyMOCK0", nil
	}

	// 1. Search in /dev/serial/by-id/
	if path, err := discoverBySerialID(); err == nil && path != "" {
		return path, nil
	}

	// 2. Scan sysfs for matching USB VIDs/PIDs
	if path, err := discoverBySysFS(); err == nil && path != "" {
		return path, nil
	}

	// 3. Fallback: scan for any active /dev/ttyUSB or /dev/ttyACM devices
	if path, err := discoverByTTY(); err == nil && path != "" {
		return path, nil
	}

	return "", fmt.Errorf("no Thread USB radio dongles could be automatically detected")
}

// discoverBySerialID searches for known Thread hardware under /dev/serial/by-id.
func discoverBySerialID() (string, error) {
	byIDPath := "/dev/serial/by-id"
	if _, err := os.Stat(byIDPath); err != nil {
		return "", err
	}

	files, err := os.ReadDir(byIDPath)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		name := strings.ToLower(file.Name())
		if isKnownHardwareName(name) {
			fullPath := filepath.Join(byIDPath, file.Name())
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err == nil {
				log.Printf("[Hardware] Zero-Config Auto-Matched: %s -> %s\n", file.Name(), resolved)
				return resolved, nil
			}
		}
	}

	return "", nil
}

// discoverBySysFS searches for matching profiles in sysfs.
func discoverBySysFS() (string, error) {
	devices := scanSysFS()
	if len(devices) > 0 {
		log.Printf("[Hardware] Discovered %d matching Thread USB hardware signatures\n", len(devices))
		for _, dev := range devices {
			log.Printf("[Hardware] Signature Match: %s (%s) - VID:%s PID:%s\n", dev.Path, dev.Description, dev.VendorID, dev.ProductID)
		}
		return devices[0].Path, nil
	}
	return "", nil
}

// discoverByTTY falls back to finding any active TTY coordinator interfaces.
func discoverByTTY() (string, error) {
	log.Println("[Hardware] No exact USB profile matched. Scanning for any active serial endpoints...")
	devFiles, err := os.ReadDir("/dev")
	if err != nil {
		return "", err
	}

	for _, f := range devFiles {
		name := f.Name()
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") {
			path := "/dev/" + name
			log.Printf("[Hardware] Auto-discovered generic serial coordinator interface: %s\n", path)
			return path, nil
		}
	}
	return "", nil
}

// isKnownHardwareName performs signature checks on file names for typical Thread modules.
func isKnownHardwareName(name string) bool {
	return strings.Contains(name, "zbt") ||
		strings.Contains(name, "skyconnect") ||
		strings.Contains(name, "sonoff") ||
		strings.Contains(name, "openthread") ||
		strings.Contains(name, "nrf52840") ||
		strings.Contains(name, "usb-serial") ||
		strings.Contains(name, "cp210") ||
		strings.Contains(name, "ch34") ||
		strings.Contains(name, "ftdi")
}

// scanSysFS audits /sys/bus/usb/devices for signature-matched hardware.
func scanSysFS() []Device {
	var matches []Device
	sysUSBDir := "/sys/bus/usb/devices"

	if _, err := os.Stat(sysUSBDir); os.IsNotExist(err) {
		return matches
	}

	walkErr := filepath.WalkDir(sysUSBDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if dev, ok := inspectSysFSDevice(path); ok {
				matches = append(matches, dev)
			}
		}
		return nil
	})
	if walkErr != nil {
		log.Printf("[Hardware] Warning: walkdir in sysFS encountered error: %v\n", walkErr)
	}

	return matches
}

// inspectSysFSDevice reads device IDs to build a Device profile if it matches target profiles.
func inspectSysFSDevice(path string) (Device, bool) {
	cleanPath := filepath.Clean(path)

	// Prevent path traversal outside system USB directories
	if !strings.HasPrefix(cleanPath, "/sys/bus/usb/devices") {
		return Device{}, false
	}

	// #nosec G304 - verified safe path prefix check above
	idVendorBytes, err1 := os.ReadFile(filepath.Join(cleanPath, "idVendor"))
	// #nosec G304 - verified safe path prefix check above
	idProductBytes, err2 := os.ReadFile(filepath.Join(cleanPath, "idProduct"))
	if err1 != nil || err2 != nil {
		return Device{}, false
	}

	vid := strings.TrimSpace(string(idVendorBytes))
	pid := strings.TrimSpace(string(idProductBytes))

	key := vid + ":" + pid
	if sig, found := targetSignatures[key]; found {
		ttyNode := findTTYNode(cleanPath)
		if ttyNode != "" {
			return Device{
				Path:        ttyNode,
				VendorID:    vid,
				ProductID:   pid,
				Description: sig.Desc,
			}, true
		}
	}

	return Device{}, false
}

// findTTYNode maps the usb device path to a local /dev tty node.
func findTTYNode(usbDevicePath string) string {
	var ttyPath string
	cleanUSBPath := filepath.Clean(usbDevicePath)

	if !strings.HasPrefix(cleanUSBPath, "/sys/bus/usb/devices") {
		return ""
	}

	walkErr := filepath.WalkDir(cleanUSBPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") {
			ttyPath = "/dev/" + name
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		log.Printf("[Hardware] Warning: walkdir finding TTY node encountered error: %v\n", walkErr)
	}

	return ttyPath
}
