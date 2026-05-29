// Package hardware implements automatic discovery and detection of USB Thread radio coordinator hardware.
//
//nolint:gosec,noctx,nestif // G301/G306 permissions are system requirements, nestif has direct container device validation
package hardware

import (
	"fmt"
	"go.bug.st/serial"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Device represents a discovered USB serial coordinator
type Device struct {
	Path        string
	VendorID    string
	ProductID   string
	Description string
	Baudrate    int
	FlowControl bool
}

// Filesystem paths used during discovery (overridable in tests).
var (
	devDir           = "/dev"
	serialByIDDir    = "/dev/serial/by-id"
	sysUSBDevicesDir = "/sys/bus/usb/devices"
)

// Known Thread coordinator signatures (VID:PID)
var targetSignatures = map[string]struct {
	Desc        string
	Baudrate    int
	FlowControl bool
}{
	// Silicon Labs (CP2102, CP2104, CP2102N, J-Link)
	"10c4:ea60": {Desc: "Silicon Labs CP210x USB to UART Bridge (Home Assistant Connect ZBT-1 / Sonoff / SkyConnect)", Baudrate: 460800, FlowControl: true},
	"10c4:ea70": {Desc: "Silicon Labs CP2105 Dual USB to UART Bridge", Baudrate: 460800, FlowControl: false},
	"10c4:ea80": {Desc: "Silicon Labs CP2108 Quad USB to UART Bridge", Baudrate: 460800, FlowControl: false},
	"1366:0101": {Desc: "Silicon Labs Segger J-Link SDK Coordinator", Baudrate: 460800, FlowControl: false},
	"1366:1015": {Desc: "Silicon Labs J-Link OB Development Link", Baudrate: 460800, FlowControl: false},

	// Nordic Semiconductor (nRF52840, nRF52833)
	"1915:528f": {Desc: "Nordic Semiconductor nRF52840 Thread Dongle (RCP)", Baudrate: 115200, FlowControl: false},
	"1915:cafe": {Desc: "Nordic Semiconductor Custom Thread/Zigbee Coordinator", Baudrate: 115200, FlowControl: false},

	// WCH (CH340, CH341, CH343 - extremely common in DIY/low-cost dongles)
	"1a86:7523": {Desc: "WCH CH340 USB to Serial Coordinator", Baudrate: 115200, FlowControl: false},
	"1a86:5523": {Desc: "WCH CH341 USB to Serial Coordinator", Baudrate: 115200, FlowControl: false},
	"1a86:55d2": {Desc: "WCH CH343 High-Speed USB to Serial Coordinator", Baudrate: 115200, FlowControl: false},

	// FTDI (FT232R, FT231X - DIY/commercial adapters)
	"0403:6001": {Desc: "FTDI FT232 USB to UART Coordinator", Baudrate: 115200, FlowControl: false},
	"0403:6015": {Desc: "FTDI FT231X USB to UART Coordinator", Baudrate: 115200, FlowControl: false},

	// Prolific (PL2303)
	"067b:2303": {Desc: "Prolific PL2303 USB to Serial Coordinator", Baudrate: 115200, FlowControl: false},
}

// SetDiscoveryPathsForTest overrides discovery filesystem roots. It returns a restore function.
func SetDiscoveryPathsForTest(dev, serialByID, sysUSB string) func() {
	oldDev, oldSerial, oldSys := devDir, serialByIDDir, sysUSBDevicesDir
	devDir = dev
	serialByIDDir = serialByID
	sysUSBDevicesDir = sysUSB
	return func() {
		devDir = oldDev
		serialByIDDir = oldSerial
		sysUSBDevicesDir = oldSys
	}
}

// DiscoverRadio looks for connected Thread RCP dongles.
// When mockMode is true, returns a simulated device path without scanning hardware.
// Returns device path, recommended baud rate (or 0 if generic), recommended flow control, and error.
func DiscoverRadio(mockMode bool) (string, int, bool, error) {
	log.Println("[Hardware] Running automatic USB serial device discovery...")

	if mockMode {
		log.Println("[Hardware] Mock mode active: returning simulated hardware path /dev/ttyMOCK0")
		return "/dev/ttyMOCK0", 460800, false, nil
	}

	// 1. Search in /dev/serial/by-id/
	if path, baud, flow, err := discoverBySerialID(); err == nil && path != "" {
		return path, baud, flow, nil
	}

	// 2. Scan sysfs for matching USB VIDs/PIDs
	if path, baud, flow, err := discoverBySysFS(); err == nil && path != "" {
		return path, baud, flow, nil
	}

	// 3. Fallback: scan for any active /dev/ttyUSB or /dev/ttyACM devices
	if path, err := discoverByTTY(); err == nil && path != "" {
		return path, 0, false, nil
	}

	return "", 0, false, fmt.Errorf("no Thread USB radio dongles could be automatically detected")
}

// discoverBySerialID searches for known Thread hardware under /dev/serial/by-id.
func discoverBySerialID() (string, int, bool, error) {
	byIDPath := serialByIDDir
	if _, err := os.Stat(byIDPath); err != nil {
		return "", 0, false, err
	}

	files, err := os.ReadDir(byIDPath)
	if err != nil {
		return "", 0, false, err
	}

	for _, file := range files {
		name := strings.ToLower(file.Name())
		if isKnownHardwareName(name) {
			fullPath := filepath.Join(byIDPath, file.Name())
			resolved, err := filepath.EvalSymlinks(fullPath)
			if err == nil {
				baud := getBaudrateFromHardwareName(name)
				flow := getFlowControlFromHardwareName(name)
				log.Printf("[Hardware] Zero-Config Auto-Matched: %s -> %s (Baudrate: %d, FlowControl: %t)\n", file.Name(), resolved, baud, flow)
				return resolved, baud, flow, nil
			}
		}
	}

	return "", 0, false, nil
}

// discoverBySysFS searches for matching profiles in sysfs.
func discoverBySysFS() (string, int, bool, error) {
	devices := scanSysFS()
	if len(devices) > 0 {
		log.Printf("[Hardware] Discovered %d matching Thread USB hardware signatures\n", len(devices))
		for _, dev := range devices {
			log.Printf("[Hardware] Signature Match: %s (%s) - VID:%s PID:%s (Baudrate: %d, FlowControl: %t)\n", dev.Path, dev.Description, dev.VendorID, dev.ProductID, dev.Baudrate, dev.FlowControl)
		}
		return devices[0].Path, devices[0].Baudrate, devices[0].FlowControl, nil
	}
	return "", 0, false, nil
}

// discoverByTTY falls back to finding any active TTY coordinator interfaces.
func discoverByTTY() (string, error) {
	log.Println("[Hardware] No exact USB profile matched. Scanning for any active serial endpoints...")
	devFiles, err := os.ReadDir(devDir)
	if err != nil {
		return "", err
	}

	for _, f := range devFiles {
		name := f.Name()
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") {
			path := filepath.Join(devDir, name)
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
	sysUSBDir := sysUSBDevicesDir

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
	if !strings.HasPrefix(cleanPath, sysUSBDevicesDir) {
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
				Baudrate:    sig.Baudrate,
				FlowControl: sig.FlowControl,
			}, true
		}
	}

	return Device{}, false
}

// findTTYNode maps the usb device path to a local /dev tty node.
func findTTYNode(usbDevicePath string) string {
	var ttyPath string
	cleanUSBPath := filepath.Clean(usbDevicePath)

	if !strings.HasPrefix(cleanUSBPath, sysUSBDevicesDir) {
		return ""
	}

	walkErr := filepath.WalkDir(cleanUSBPath, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "ttyUSB") || strings.HasPrefix(name, "ttyACM") {
			ttyPath = filepath.Join(devDir, name)
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		log.Printf("[Hardware] Warning: walkdir finding TTY node encountered error: %v\n", walkErr)
	}

	return ttyPath
}

// getBaudrateFromHardwareName maps typical smart home USB dongle names to their recommended baud rates.
func getBaudrateFromHardwareName(name string) int {
	name = strings.ToLower(name)
	switch {
	case strings.Contains(name, "zbt") ||
		strings.Contains(name, "skyconnect") ||
		strings.Contains(name, "sonoff") ||
		strings.Contains(name, "cp210"):
		return 460800
	case strings.Contains(name, "nrf52840") ||
		strings.Contains(name, "ch34") ||
		strings.Contains(name, "ftdi") ||
		strings.Contains(name, "pl2303"):
		return 115200
	default:
		return 0
	}
}

// getFlowControlFromHardwareName maps typical smart home USB dongle names to their recommended hardware flow control setting.
func getFlowControlFromHardwareName(name string) bool {
	name = strings.ToLower(name)
	// ZBT-1, SkyConnect, and other CP2102-based Silicon Labs coordinators recommend flow control
	return strings.Contains(name, "zbt") ||
		strings.Contains(name, "skyconnect") ||
		strings.Contains(name, "silabs")
}

//

// HostAudit captures host-level routing and TUN readiness for border routing.
type HostAudit struct {
	IPv6ForwardingAll     bool     `json:"ipv6ForwardingAll"`
	IPv6ForwardingDefault bool     `json:"ipv6ForwardingDefault"`
	IPv6AcceptRaAll       bool     `json:"ipv6AcceptRaAll"`
	IPv6AcceptRaDefault   bool     `json:"ipv6AcceptRaDefault"`
	TunDeviceExists       bool     `json:"tunDeviceExists"`
	Warnings              []string `json:"warnings"`
}

func tryConfigureSysctl(path, value string) {
	if checkSysctl(path, value) {
		log.Printf("[Hardware] Host %s is already set to %s\n", path, value)
		return
	}
	log.Printf("[Hardware] Attempting to auto-configure %s to %s...\n", path, value)
	if err := writeSysctl(path, value); err != nil {
		log.Printf("[Hardware] Warning: failed to auto-configure %s to %s: %v\n", path, value, err)
	} else {
		log.Printf("[Hardware] Successfully auto-configured %s to %s\n", path, value)
	}
}

func tryConfigureTunDevice() {
	if _, err := os.Stat("/dev/net/tun"); os.IsNotExist(err) {
		log.Println("[Hardware] Virtual TUN device node /dev/net/tun is missing. Attempting auto-creation...")
		if err := os.MkdirAll("/dev/net", 0755); err != nil {
			log.Printf("[Hardware] Warning: failed to create directory /dev/net: %v\n", err)
		} else {
			cmd := exec.Command("mknod", "/dev/net/tun", "c", "10", "200")
			if err := cmd.Run(); err != nil {
				log.Printf("[Hardware] Warning: failed to create /dev/net/tun via mknod: %v\n", err)
			} else {
				log.Println("[Hardware] Successfully created virtual TUN device node /dev/net/tun")
			}
		}
	} else {
		log.Println("[Hardware] Virtual TUN device node /dev/net/tun exists and is accessible")
	}
}

// SelfHealHost attempts to configure the host for optimal routing and TUN readiness.
func SelfHealHost() {
	log.Println("[Hardware] Running self-healing steps for host networking configuration...")

	// 1. Try to enable IPv6 forwarding if disabled
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/all/forwarding", "1")
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/default/forwarding", "1")

	// 2. Try to set accept_ra to 2 if not set
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/all/accept_ra", "2")
	tryConfigureSysctl("/proc/sys/net/ipv6/conf/default/accept_ra", "2")

	// 3. Try to auto-create /dev/net/tun if missing
	tryConfigureTunDevice()
}

// AuditHost checks host-level routing configurations and virtual interface capabilities.
// When mockMode is true (integration tests without host network), IPv6 sysctl checks are skipped
// because bridge containers cannot write /proc/sys and would only produce misleading warnings.
func AuditHost(mockMode bool) HostAudit {
	if !mockMode {
		SelfHealHost()
	}
	var audit HostAudit
	if mockMode {
		_, err := os.Stat("/dev/net/tun")
		audit.TunDeviceExists = err == nil
		return audit
	}
	audit.IPv6ForwardingAll = checkSysctl("/proc/sys/net/ipv6/conf/all/forwarding", "1")
	audit.IPv6ForwardingDefault = checkSysctl("/proc/sys/net/ipv6/conf/default/forwarding", "1")

	if !audit.IPv6ForwardingAll || !audit.IPv6ForwardingDefault {
		audit.Warnings = append(audit.Warnings, "\n"+
			"================================================================================\n"+
			"[DIAGNOSTIC REPORT] HOST IPV6 FORWARDING DISABLED\n"+
			"================================================================================\n"+
			"Issue: Host IPv6 packet forwarding is disabled.\n"+
			"Root Cause: The host kernel is not routing IPv6 packets, which prevents Thread border routing.\n"+
			"How to Fix: Run the following command on your host machine:\n"+
			"  sysctl -w net.ipv6.conf.all.forwarding=1 net.ipv6.conf.default.forwarding=1\n"+
			"================================================================================")
	}

	audit.IPv6AcceptRaAll = checkSysctl("/proc/sys/net/ipv6/conf/all/accept_ra", "2")
	audit.IPv6AcceptRaDefault = checkSysctl("/proc/sys/net/ipv6/conf/default/accept_ra", "2")

	if !audit.IPv6AcceptRaAll || !audit.IPv6AcceptRaDefault {
		audit.Warnings = append(audit.Warnings, "\n"+
			"================================================================================\n"+
			"[DIAGNOSTIC REPORT] HOST IPV6 ACCEPT_RA NOT SET TO 2\n"+
			"================================================================================\n"+
			"Issue: Host Accept Router Advertisements (accept_ra) is not configured to 2.\n"+
			"Root Cause: The host is not configured to accept Router Advertisements when forwarding is enabled.\n"+
			"How to Fix: Run the following command on your host machine:\n"+
			"  sysctl -w net.ipv6.conf.all.accept_ra=2 net.ipv6.conf.default.accept_ra=2\n"+
			"================================================================================")
	}

	_, err := os.Stat("/dev/net/tun")
	audit.TunDeviceExists = (err == nil)
	if !audit.TunDeviceExists {
		audit.Warnings = append(audit.Warnings, "\n"+
			"================================================================================\n"+
			"[DIAGNOSTIC REPORT] MISSING VIRTUAL TUN DEVICE\n"+
			"================================================================================\n"+
			"Issue: Virtual TUN adapter device /dev/net/tun is missing in the container.\n"+
			"Root Cause: The container is running without proper Linux capabilities or device permissions.\n"+
			"How to Fix: Start the container with NET_ADMIN capability and mount the TUN device:\n"+
			"  Docker Run:   --cap-add=NET_ADMIN --device /dev/net/tun\n"+
			"  Compose file:\n"+
			"    cap_add:\n"+
			"      - NET_ADMIN\n"+
			"    devices:\n"+
			"      - /dev/net/tun:/dev/net/tun\n"+
			"================================================================================")
	}

	return audit
}

func checkSysctl(path, expected string) bool {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is a fixed sysctl location from AuditHost
	if err != nil {
		return false
	}
	val := strings.TrimSpace(string(data))
	return val == expected
}

func writeSysctl(path, value string) error {
	return os.WriteFile(path, []byte(value+"\n"), 0644)
}

const (
	kFlagXOn        = 0x11
	kFlagXOff       = 0x13
	kFlagSequence   = 0x7e
	kEscapeSequence = 0x7d
	kFlagSpecial    = 0xf8
)

var fcsTable [256]uint16

// serialOpen opens a serial port (overridable in tests).
var serialOpen = serial.Open

func init() {
	for i := range uint16(256) {
		entry := i
		for range 8 {
			if (entry & 1) != 0 {
				entry = (entry >> 1) ^ 0x8408
			} else {
				entry >>= 1
			}
		}
		fcsTable[i] = entry
	}
}

// UpdateFcs calculates the running FCS checksum.
func UpdateFcs(fcs uint16, b byte) uint16 {
	return (fcs >> 8) ^ fcsTable[(fcs^uint16(b))&0xff]
}

// HdlcByteNeedsEscape checks if a byte must be escaped under HDLC-Lite.
func HdlcByteNeedsEscape(b byte) bool {
	return b == kFlagSequence || b == kEscapeSequence || b == kFlagXOn || b == kFlagXOff || b == kFlagSpecial
}

func appendEscaped(out []byte, b byte) []byte {
	if HdlcByteNeedsEscape(b) {
		return append(out, kEscapeSequence, b^0x20)
	}
	return append(out, b)
}

// EncodeHdlc wraps a raw Spinel payload into an HDLC-lite framed packet.
func EncodeHdlc(payload []byte) []byte {
	var out []byte
	out = append(out, kFlagSequence)

	fcs := uint16(0xffff)
	for _, b := range payload {
		fcs = UpdateFcs(fcs, b)
		out = appendEscaped(out, b)
	}

	fcs ^= 0xffff
	out = appendEscaped(out, byte(fcs&0xff))
	out = appendEscaped(out, byte(fcs>>8))

	return append(out, kFlagSequence)
}

func findHdlcStart(data []byte) int {
	for i, b := range data {
		if b == kFlagSequence {
			return i
		}
	}
	return -1
}

func unescapeHdlcFrame(data []byte, startIdx int) ([]byte, int) {
	var unescaped []byte
	escaped := false
	endIdx := -1

	for i := startIdx + 1; i < len(data); i++ {
		b := data[i]
		if b == kFlagSequence {
			if len(unescaped) > 0 {
				return unescaped, i
			}
			unescaped = unescaped[:0]
			escaped = false
			continue
		}
		if b == kEscapeSequence {
			escaped = true
			continue
		}
		if escaped {
			b ^= 0x20
			escaped = false
		}
		unescaped = append(unescaped, b)
	}
	return unescaped, endIdx
}

func validateHdlcFCS(unescaped []byte) bool {
	fcs := uint16(0xffff)
	for _, b := range unescaped {
		fcs = UpdateFcs(fcs, b)
	}
	return fcs == 0xf0b8
}

// DecodeHdlc unescapes and validates the checksum of an HDLC frame.
// Returns unescaped payload on success.
func DecodeHdlc(data []byte) ([]byte, bool) {
	if len(data) < 4 {
		return nil, false
	}

	startIdx := findHdlcStart(data)
	if startIdx == -1 {
		return nil, false
	}

	unescaped, endIdx := unescapeHdlcFrame(data, startIdx)
	if endIdx == -1 || len(unescaped) < 2 || !validateHdlcFCS(unescaped) {
		return nil, false
	}

	return unescaped[:len(unescaped)-2], true
}

func parseNCPVersionPayload(payload []byte) (string, bool) {
	if len(payload) < 3 || payload[1] != 0x06 || payload[2] != 0x02 {
		return "", false
	}
	versionStr := string(payload[3:])
	for len(versionStr) > 0 && versionStr[len(versionStr)-1] == 0 {
		versionStr = versionStr[:len(versionStr)-1]
	}
	return versionStr, true
}

// ProbeDevice performs a pre-flight Spinel NCP_VERSION GET check on a serial port.
func ProbeDevice(portPath string, baudrate int) (string, error) {
	log.Printf("[Hardware] Opening serial port %s for pre-flight Spinel probe at baudrate %d...\n", portPath, baudrate)
	mode := &serial.Mode{
		BaudRate: baudrate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serialOpen(portPath, mode)
	if err != nil {
		log.Printf("[Hardware] Probe error: failed to open serial port %s: %v\n", portPath, err)
		return "", fmt.Errorf("failed to open serial port %s: %w", portPath, err)
	}
	defer func() {
		log.Printf("[Hardware] Closing serial port %s probe connection...\n", portPath)
		_ = port.Close()
	}()

	// Clear local buffers
	_ = port.ResetInputBuffer()
	_ = port.ResetOutputBuffer()

	// Spinel: Header (0x81), Command GET (0x02), Property NCP_VERSION (0x02)
	spinelCmd := []byte{0x81, 0x02, 0x02}
	hdlcFrame := EncodeHdlc(spinelCmd)

	log.Printf("[Hardware] Writing Spinel NCP_VERSION GET query HDLC frame (%d bytes) to %s...\n", len(hdlcFrame), portPath)
	_, err = port.Write(hdlcFrame)
	if err != nil {
		log.Printf("[Hardware] Probe error: failed to write to serial port %s: %v\n", portPath, err)
		return "", fmt.Errorf("failed to write probe frame: %w", err)
	}

	version, err := readProbeResponse(port)
	if err != nil {
		log.Printf("[Hardware] Probe error: failed to read valid response from %s: %v\n", portPath, err)
		return "", err
	}
	log.Printf("[Hardware] Probe success: discovered firmware version: %s\n", version)
	return version, nil
}

func readProbeResponse(port serial.Port) (string, error) {
	readBuf := make([]byte, 1024)
	var rawData []byte
	deadline := time.Now().Add(2 * time.Second)
	log.Println("[Hardware] Waiting up to 2 seconds for Spinel NCP_VERSION response...")

	for time.Now().Before(deadline) {
		_ = port.SetReadTimeout(200 * time.Millisecond)
		n, err := port.Read(readBuf)
		if err != nil || n <= 0 {
			continue
		}
		rawData = append(rawData, readBuf[:n]...)
		log.Printf("[Hardware] Read %d bytes from serial port, total accumulated payload: %d bytes\n", n, len(rawData))
		payload, ok := DecodeHdlc(rawData)
		if !ok {
			continue
		}
		if version, ok := parseNCPVersionPayload(payload); ok {
			return version, nil
		}
		log.Printf("[Hardware] Received valid HDLC frame but payload was not a valid NCP_VERSION response: %v\n", payload)
	}

	return "", fmt.Errorf("spinel probe timed out or returned invalid response (detected CPC/MultiPAN or incorrect firmware)")
}
