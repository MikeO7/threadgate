package hardware

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"go.bug.st/serial"
)

func TestHdlcEncodeDecode(t *testing.T) {
	payload := []byte{0x81, 0x02, 0x02}

	encoded := EncodeHdlc(payload)

	if len(encoded) < 7 {
		t.Fatalf("Encoded frame too short: %x", encoded)
	}
	if encoded[0] != kFlagSequence || encoded[len(encoded)-1] != kFlagSequence {
		t.Errorf("Invalid start/end flags: %x", encoded)
	}

	decoded, ok := DecodeHdlc(encoded)
	if !ok {
		t.Fatalf("Failed to decode framed packet")
	}

	if !bytes.Equal(decoded, payload) {
		t.Errorf("Decoded payload mismatch: got %x, want %x", decoded, payload)
	}
}

func TestHdlcEscaping(t *testing.T) {
	payload := []byte{0x81, kFlagSequence, kEscapeSequence, kFlagXOn, kFlagXOff, kFlagSpecial}

	encoded := EncodeHdlc(payload)
	decoded, ok := DecodeHdlc(encoded)
	if !ok {
		t.Fatalf("Failed to decode escaped framed packet")
	}

	if !bytes.Equal(decoded, payload) {
		t.Errorf("Decoded payload mismatch: got %x, want %x", decoded, payload)
	}
}

func TestFindHdlcStart(t *testing.T) {
	if idx := findHdlcStart([]byte{0x00, 0x01}); idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
	if idx := findHdlcStart([]byte{0x00, kFlagSequence, 0x02}); idx != 1 {
		t.Errorf("expected 1, got %d", idx)
	}
}

func TestDecodeHdlcInvalid(t *testing.T) {
	if _, ok := DecodeHdlc([]byte{0x01, 0x02}); ok {
		t.Error("expected decode failure for short frame")
	}
	if _, ok := DecodeHdlc([]byte{0x00, 0x01, 0x02, 0x03}); ok {
		t.Error("expected decode failure without flag")
	}
}

func TestParseNCPVersionPayload(t *testing.T) {
	payload := append([]byte{0x81, 0x06, 0x02}, []byte("OpenThread/1.0.0\x00")...)
	version, ok := parseNCPVersionPayload(payload)
	if !ok {
		t.Fatal("expected valid NCP version payload")
	}
	if version != "OpenThread/1.0.0" {
		t.Errorf("unexpected version: %q", version)
	}

	if _, ok := parseNCPVersionPayload([]byte{0x81, 0x02, 0x02}); ok {
		t.Error("expected invalid payload to fail")
	}
}

type mockSerialPort struct {
	readData []byte
	readPos  int
	writeErr error
}

func (m *mockSerialPort) SetMode(_ *serial.Mode) error { return nil }
func (m *mockSerialPort) Read(p []byte) (int, error) {
	if m.readPos >= len(m.readData) {
		return 0, nil
	}
	n := copy(p, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}
func (m *mockSerialPort) Write(_ []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return 0, nil
}
func (m *mockSerialPort) Drain() error                { return nil }
func (m *mockSerialPort) ResetInputBuffer() error     { return nil }
func (m *mockSerialPort) ResetOutputBuffer() error    { return nil }
func (m *mockSerialPort) SetDTR(_ bool) error         { return nil }
func (m *mockSerialPort) SetRTS(_ bool) error         { return nil }
func (m *mockSerialPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}
func (m *mockSerialPort) SetReadTimeout(_ time.Duration) error { return nil }
func (m *mockSerialPort) Close() error                         { return nil }
func (m *mockSerialPort) Break(_ time.Duration) error          { return nil }

func TestReadProbeResponse(t *testing.T) {
	versionPayload := append([]byte{0x81, 0x06, 0x02}, []byte("MockRCP/1.0")...)
	frame := EncodeHdlc(versionPayload)

	port := &mockSerialPort{readData: frame}
	version, err := readProbeResponse(port)
	if err != nil {
		t.Fatalf("readProbeResponse failed: %v", err)
	}
	if version != "MockRCP/1.0" {
		t.Errorf("unexpected version: %q", version)
	}
}

func TestReadProbeResponseTimeout(t *testing.T) {
	port := &mockSerialPort{}
	_, err := readProbeResponse(port)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestProbeDeviceOpenFailure(t *testing.T) {
	_, err := ProbeDevice("/dev/ttyDOESNOTEXIST999", 115200)
	if err == nil {
		t.Fatal("expected open failure")
	}
}

func TestProbeDeviceWriteFailure(t *testing.T) {
	oldOpen := serialOpen
	serialOpen = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return &mockSerialPort{writeErr: fmt.Errorf("write failed")}, nil
	}
	t.Cleanup(func() { serialOpen = oldOpen })

	_, err := ProbeDevice("/dev/ttyMOCK0", 115200)
	if err == nil {
		t.Fatal("expected write failure")
	}
}

func TestUnescapeHdlcFrame(t *testing.T) {
	// Inner flag with no payload resets the accumulator.
	data := []byte{kFlagSequence, kFlagSequence, kFlagSequence}
	unescaped, endIdx := unescapeHdlcFrame(data, 0)
	if len(unescaped) != 0 || endIdx != -1 {
		t.Fatalf("unexpected reset result: %v idx=%d", unescaped, endIdx)
	}

	// Incomplete frame without closing flag.
	payload := []byte{0x81, 0x02}
	open := append([]byte{kFlagSequence}, payload...)
	unescaped, endIdx = unescapeHdlcFrame(open, 0)
	if endIdx != -1 || len(unescaped) != len(payload) {
		t.Fatalf("expected open frame bytes %v idx=%d", unescaped, endIdx)
	}
}

func TestDecodeHdlcBadFCS(t *testing.T) {
	bad := []byte{kFlagSequence, 0x81, 0x02, 0x02, 0x00, 0x00, kFlagSequence}
	if _, ok := DecodeHdlc(bad); ok {
		t.Fatal("expected FCS validation failure")
	}
}

func TestReadProbeResponseInvalidPayload(t *testing.T) {
	frame := EncodeHdlc([]byte{0x81, 0x02, 0x02})
	port := &mockSerialPort{readData: frame}
	_, err := readProbeResponse(port)
	if err == nil {
		t.Fatal("expected invalid payload error")
	}
}

func TestProbeDeviceSuccess(t *testing.T) {
	versionPayload := append([]byte{0x81, 0x06, 0x02}, []byte("ProbeOK/1.0")...)
	frame := EncodeHdlc(versionPayload)

	oldOpen := serialOpen
	serialOpen = func(_ string, _ *serial.Mode) (serial.Port, error) {
		return &mockSerialPort{readData: frame}, nil
	}
	t.Cleanup(func() { serialOpen = oldOpen })

	version, err := ProbeDevice("/dev/ttyMOCK0", 115200)
	if err != nil {
		t.Fatalf("ProbeDevice failed: %v", err)
	}
	if version != "ProbeOK/1.0" {
		t.Errorf("unexpected version: %q", version)
	}
}