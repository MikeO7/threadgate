package thread

import (
	"encoding/hex"
	"testing"
)

func TestMockNetworkKeyNotOpenThreadWebUIDefault(t *testing.T) {
	t.Helper()
	// homeassistant/components/otbr/util.py INSECURE_NETWORK_KEYS
	insecure := "00112233445566778899aabbccddeeff"
	if MockNetworkKeyHex == insecure {
		t.Fatal("mock key must not match HA insecure default")
	}
	key, err := hex.DecodeString(MockNetworkKeyHex)
	if err != nil || len(key) != 16 {
		t.Fatalf("mock key: %v len=%d", err, len(key))
	}
	parsed, err := BuildOperationalDatasetHex(MockNetworkKeyHex, 1)
	if err != nil {
		t.Fatal(err)
	}
	if DatasetContainsInsecureNetworkKey(parsed) {
		t.Fatal("dataset still embeds insecure key")
	}
	if !DatasetContainsInsecureNetworkKey("0510" + InsecureDefaultNetworkKeyHex) {
		t.Fatal("expected insecure key detection")
	}
}
