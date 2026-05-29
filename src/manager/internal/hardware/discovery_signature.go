package hardware

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectUSBSerialSignature returns a human-readable coordinator description and USB IDs
// using Linux sysfs and /dev/serial/by-id.
func DetectUSBSerialSignature(devicePath string) (desc, vid, pid string, found bool) {
	if devicePath == "" {
		return "", "", "", false
	}
	if desc, vid, pid, found = describeDeviceFromSysFS(devicePath); found {
		return desc, vid, pid, true
	}
	return describeDeviceFromSerialByID(devicePath)
}

func describeDeviceFromSysFS(devicePath string) (desc, vid, pid string, found bool) {
	cleanPath := filepath.Clean(devicePath)
	for _, dev := range scanSysFS() {
		if filepath.Clean(dev.Path) != cleanPath {
			continue
		}
		description := dev.Description
		if name, ok := serialByIDNameForPath(cleanPath); ok {
			description = preferSerialByIDDescription(name, dev.Description)
		}
		return description, dev.VendorID, dev.ProductID, true
	}
	return "", "", "", false
}

func describeDeviceFromSerialByID(devicePath string) (desc, vid, pid string, found bool) {
	name, ok := serialByIDNameForPath(devicePath)
	if !ok {
		return "", "", "", false
	}
	desc = parseSerialByIDProductName(name)
	if desc == "" {
		return "", "", "", false
	}
	cleanPath := filepath.Clean(devicePath)
	for _, dev := range scanSysFS() {
		if filepath.Clean(dev.Path) == cleanPath {
			return preferSerialByIDDescription(name, desc), dev.VendorID, dev.ProductID, true
		}
	}
	return desc, "", "", true
}

func serialByIDNameForPath(devicePath string) (string, bool) {
	byIDPath := serialByIDDir
	if _, err := os.Stat(byIDPath); err != nil {
		return "", false
	}
	entries, err := os.ReadDir(byIDPath)
	if err != nil {
		return "", false
	}
	cleanTarget := filepath.Clean(devicePath)
	resolvedTarget, err := filepath.EvalSymlinks(cleanTarget)
	if err != nil {
		resolvedTarget = cleanTarget
	}
	for _, entry := range entries {
		linkPath := filepath.Join(byIDPath, entry.Name())
		resolved, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			continue
		}
		if filepath.Clean(resolved) == cleanTarget || filepath.Clean(resolved) == resolvedTarget {
			return entry.Name(), true
		}
	}
	return "", false
}

func preferSerialByIDDescription(byIDName, fallback string) string {
	if parsed := parseSerialByIDProductName(byIDName); parsed != "" {
		return parsed
	}
	return fallback
}

func parseSerialByIDProductName(byIDName string) string {
	name := strings.TrimPrefix(byIDName, "usb-")
	if idx := strings.Index(name, "-if"); idx > 0 {
		name = name[:idx]
	}
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return strings.ReplaceAll(name, "_", " ")
	}
	// Drop trailing USB serial hex blob when present.
	last := parts[len(parts)-1]
	if len(last) >= 16 && isHexString(last) {
		parts = parts[:len(parts)-1]
	}
	if len(parts) >= 2 && strings.EqualFold(parts[0], parts[1]) {
		parts = parts[1:]
	}
	return strings.Join(parts, " ")
}

func isHexString(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return len(value) > 0
}

// FormatDetectedDevice builds the dashboard display string for a discovered coordinator.
func FormatDetectedDevice(desc, vid, pid string) string {
	if desc == "" {
		return ""
	}
	if vid != "" && pid != "" {
		return fmt.Sprintf("%s (VID: %s, PID: %s)", desc, vid, pid)
	}
	return desc
}
