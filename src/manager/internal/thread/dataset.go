package thread

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// TestNetworkName is the operational dataset network name used in tests and mock mode.
const TestNetworkName = "Thread-Test"

// InsecureDefaultNetworkKeyHex is the OpenThread web UI default key flagged by Home Assistant.
const InsecureDefaultNetworkKeyHex = "00112233445566778899aabbccddeeff"

// MockNetworkKeyHex is the simulated Thread network key (not the OpenThread web UI default).
const MockNetworkKeyHex = "a1b2c3d4e5f60718293a4b5c6d7e8f90"

// ValidOperationalDatasetHex is a complete Thread operational dataset TLV suitable for
// Home Assistant's thread dataset store (channel, PAN ID, ext PAN ID, name, key, prefix, timestamp).
// Active timestamp 2 so HA replaces datasets imported earlier with the old insecure mock key.
var ValidOperationalDatasetHex = mustBuildOperationalDatasetHex(MockNetworkKeyHex, 2)

// DatasetContainsInsecureNetworkKey reports whether hex TLV embeds HA's flagged default network key.
func DatasetContainsInsecureNetworkKey(hexTLV string) bool {
	return strings.Contains(strings.ToLower(hexTLV), InsecureDefaultNetworkKeyHex)
}

// BuildOperationalDatasetHex encodes a minimal operational dataset for tests and mock mode.
func BuildOperationalDatasetHex(networkKeyHex string, activeTimestamp uint64) (string, error) {
	key, err := hex.DecodeString(networkKeyHex)
	if err != nil || len(key) != 16 {
		return "", fmt.Errorf("network key must be 16 bytes hex: %w", err)
	}
	parts := []byte{
		0, 3, 0, 0, 15, // channel 15
		1, 2, 0x12, 0x34,
	}
	parts = append(parts, 2, 8)
	parts = append(parts, mustHex("1122334455667788")...)
	name := []byte(TestNetworkName)
	nameLen := len(name)
	if nameLen > 255 {
		return "", fmt.Errorf("network name exceeds TLV length limit")
	}
	parts = append(parts, 3, uint8(nameLen))
	parts = append(parts, name...)
	parts = append(parts, 5, 16)
	parts = append(parts, key...)
	parts = append(parts, 7, 8)
	parts = append(parts, mustHex("fd000db800000000")...)
	parts = append(parts, 14, 8)
	ts := make([]byte, 8)
	for i := range 8 {
		shift := uint(8 * i)
		if shift >= 64 {
			break
		}
		ts[7-i] = byte((activeTimestamp >> shift) & 0xff)
	}
	parts = append(parts, ts...)
	return hex.EncodeToString(parts), nil
}

func mustBuildOperationalDatasetHex(networkKeyHex string, activeTimestamp uint64) string {
	s, err := BuildOperationalDatasetHex(networkKeyHex, activeTimestamp)
	if err != nil {
		panic(err)
	}
	return s
}

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
