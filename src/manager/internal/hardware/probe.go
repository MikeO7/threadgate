package hardware

import (
	"fmt"
	"time"

	"go.bug.st/serial"
)

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
	for i := 0; i < 256; i++ {
		entry := uint16(i)
		for j := 0; j < 8; j++ {
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

// EncodeHdlc wraps a raw Spinel payload into an HDLC-lite framed packet.
func EncodeHdlc(payload []byte) []byte {
	var out []byte
	out = append(out, kFlagSequence)

	fcs := uint16(0xffff)
	for _, b := range payload {
		fcs = UpdateFcs(fcs, b)
		if HdlcByteNeedsEscape(b) {
			out = append(out, kEscapeSequence, b^0x20)
		} else {
			out = append(out, b)
		}
	}

	// Finalize FCS
	fcs ^= 0xffff
	fcsLow := byte(fcs & 0xff)
	fcsHigh := byte(fcs >> 8)

	// Encode FCS low byte
	if HdlcByteNeedsEscape(fcsLow) {
		out = append(out, kEscapeSequence, fcsLow^0x20)
	} else {
		out = append(out, fcsLow)
	}

	// Encode FCS high byte
	if HdlcByteNeedsEscape(fcsHigh) {
		out = append(out, kEscapeSequence, fcsHigh^0x20)
	} else {
		out = append(out, fcsHigh)
	}

	out = append(out, kFlagSequence)
	return out
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
	mode := &serial.Mode{
		BaudRate: baudrate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serialOpen(portPath, mode)
	if err != nil {
		return "", fmt.Errorf("failed to open serial port %s: %w", portPath, err)
	}
	defer func() { _ = port.Close() }()

	// Clear local buffers
	_ = port.ResetInputBuffer()
	_ = port.ResetOutputBuffer()

	// Spinel: Header (0x81), Command GET (0x02), Property NCP_VERSION (0x02)
	spinelCmd := []byte{0x81, 0x02, 0x02}
	hdlcFrame := EncodeHdlc(spinelCmd)

	_, err = port.Write(hdlcFrame)
	if err != nil {
		return "", fmt.Errorf("failed to write probe frame: %w", err)
	}

	version, err := readProbeResponse(port)
	if err != nil {
		return "", err
	}
	return version, nil
}

func readProbeResponse(port serial.Port) (string, error) {
	readBuf := make([]byte, 1024)
	var rawData []byte
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		_ = port.SetReadTimeout(200 * time.Millisecond)
		n, err := port.Read(readBuf)
		if err != nil || n <= 0 {
			continue
		}
		rawData = append(rawData, readBuf[:n]...)
		payload, ok := DecodeHdlc(rawData)
		if !ok {
			continue
		}
		if version, ok := parseNCPVersionPayload(payload); ok {
			return version, nil
		}
	}

	return "", fmt.Errorf("spinel probe timed out or returned invalid response (detected CPC/MultiPAN or incorrect firmware)")
}
