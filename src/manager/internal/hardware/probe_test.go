package hardware

import (
	"bytes"
	"testing"
)

func TestHdlcEncodeDecode(t *testing.T) {
	// Let's take a raw payload
	payload := []byte{0x81, 0x02, 0x02}

	// Encode it
	encoded := EncodeHdlc(payload)

	// Verify it has start/end flags
	if len(encoded) < 7 {
		t.Fatalf("Encoded frame too short: %x", encoded)
	}
	if encoded[0] != kFlagSequence || encoded[len(encoded)-1] != kFlagSequence {
		t.Errorf("Invalid start/end flags: %x", encoded)
	}

	// Decode it
	decoded, ok := DecodeHdlc(encoded)
	if !ok {
		t.Fatalf("Failed to decode framed packet")
	}

	if !bytes.Equal(decoded, payload) {
		t.Errorf("Decoded payload mismatch: got %x, want %x", decoded, payload)
	}
}

func TestHdlcEscaping(t *testing.T) {
	// Payload with bytes that must be escaped
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
