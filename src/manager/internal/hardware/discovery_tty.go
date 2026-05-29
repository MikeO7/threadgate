package hardware

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

var genericSerialTTYPrefixes = []string{
	"ttyUSB", "ttyACM",
	"cu.usbserial-", "tty.usbserial-",
	"cu.usbmodem", "tty.usbmodem",
}

// isGenericSerialTTY reports whether a /dev node name is a USB serial coordinator endpoint.
func isGenericSerialTTY(name string) bool {
	for _, prefix := range genericSerialTTYPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// preferSerialDevice picks the best serial path when multiple endpoints exist.
// On macOS, cu.* call-out ports are preferred over tty.*.
func preferSerialDevice(paths []string) string {
	for _, path := range paths {
		base := filepath.Base(path)
		if strings.HasPrefix(base, "cu.usbserial-") || strings.HasPrefix(base, "cu.usbmodem") {
			return path
		}
	}
	return paths[0]
}

// discoverByTTY falls back to finding any active TTY coordinator interfaces.
func discoverByTTY() (path string, baud int, flow bool, err error) {
	log.Println("[Hardware] No exact USB profile matched. Scanning for any active serial endpoints...")
	devFiles, err := os.ReadDir(devDir)
	if err != nil {
		return "", 0, false, err
	}

	var candidates []string
	for _, f := range devFiles {
		name := f.Name()
		if isGenericSerialTTY(name) {
			candidates = append(candidates, filepath.Join(devDir, name))
		}
	}
	if len(candidates) == 0 {
		return "", 0, false, nil
	}
	path = preferSerialDevice(candidates)
	baud, flow = recommendedTTYSettings(path)
	log.Printf("[Hardware] Auto-discovered generic serial coordinator interface: %s (Baudrate: %d, FlowControl: %t)\n", path, baud, flow)
	return path, baud, flow, nil
}

// recommendedTTYSettings resolves baud and flow control for a generic serial path.
func recommendedTTYSettings(devicePath string) (baud int, flow bool) {
	if baud, flow, ok := coordinatorSettingsFromMacIOReg(); ok {
		return baud, flow
	}
	name := strings.ToLower(filepath.Base(devicePath))
	return getBaudrateFromHardwareName(name), getFlowControlFromHardwareName(name)
}
