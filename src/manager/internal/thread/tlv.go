package thread

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	tlvTypeChannel         byte = 0
	tlvTypePanID           byte = 1
	tlvTypeExtPanID        byte = 2
	tlvTypeNetworkName     byte = 3
	tlvTypeNetworkKey      byte = 5
	tlvTypeMeshLocalPrefix byte = 7
	tlvTypeActiveTimestamp byte = 14
)

// DecodedDataset represents the human-readable network credentials parsed from a hex TLV dataset.
type DecodedDataset struct {
	NetworkName     string `json:"network_name,omitempty"`
	Channel         uint16 `json:"channel,omitempty"`
	PanID           string `json:"pan_id,omitempty"`
	ExtPanID        string `json:"ext_pan_id,omitempty"`
	NetworkKey      string `json:"network_key,omitempty"`
	MeshLocalPrefix string `json:"mesh_local_prefix,omitempty"`
	ActiveTimestamp uint64 `json:"active_timestamp,omitempty"`
}

// DecodeDataset parses a raw hex-encoded Thread Active Operational Dataset into a DecodedDataset.
func DecodeDataset(hexStr string) (DecodedDataset, error) {
	hexStr = strings.TrimSpace(hexStr)
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return DecodedDataset{}, fmt.Errorf("invalid hex string: %w", err)
	}

	var decoded DecodedDataset
	for idx := 0; idx < len(data); {
		entry, next, ok := readTLVEntry(data, idx)
		if !ok {
			break
		}
		applyTLVField(&decoded, entry)
		idx = next
	}
	return decoded, nil
}

type tlvEntry struct {
	typ byte
	val []byte
}

func readTLVEntry(data []byte, idx int) (tlvEntry, int, bool) {
	if idx+2 > len(data) {
		return tlvEntry{}, idx, false
	}
	length := int(data[idx+1])
	next := idx + 2 + length
	if next > len(data) {
		return tlvEntry{}, idx, false
	}
	return tlvEntry{typ: data[idx], val: data[idx+2 : next]}, next, true
}

func applyTLVField(decoded *DecodedDataset, entry tlvEntry) {
	switch entry.typ {
	case tlvTypeChannel:
		decodeChannelField(decoded, entry.val)
	case tlvTypePanID:
		decodePanIDField(decoded, entry.val)
	case tlvTypeExtPanID:
		decodeExtPanIDField(decoded, entry.val)
	case tlvTypeNetworkName:
		decoded.NetworkName = string(entry.val)
	case tlvTypeNetworkKey:
		decodeNetworkKeyField(decoded, entry.val)
	case tlvTypeMeshLocalPrefix:
		decodeMeshLocalPrefixField(decoded, entry.val)
	case tlvTypeActiveTimestamp:
		decodeActiveTimestampField(decoded, entry.val)
	}
}

func decodeChannelField(decoded *DecodedDataset, val []byte) {
	if len(val) >= 3 {
		decoded.Channel = binary.BigEndian.Uint16(val[1:3])
	}
}

func decodePanIDField(decoded *DecodedDataset, val []byte) {
	if len(val) == 2 {
		decoded.PanID = fmt.Sprintf("0x%04x", binary.BigEndian.Uint16(val))
	}
}

func decodeExtPanIDField(decoded *DecodedDataset, val []byte) {
	if len(val) == 8 {
		decoded.ExtPanID = hex.EncodeToString(val)
	}
}

func decodeNetworkKeyField(decoded *DecodedDataset, val []byte) {
	if len(val) == 16 {
		decoded.NetworkKey = hex.EncodeToString(val)
	}
}

func decodeMeshLocalPrefixField(decoded *DecodedDataset, val []byte) {
	if len(val) == 8 {
		decoded.MeshLocalPrefix = fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x::/64",
			val[0], val[1], val[2], val[3], val[4], val[5], val[6], val[7])
	}
}

func decodeActiveTimestampField(decoded *DecodedDataset, val []byte) {
	if len(val) == 8 {
		decoded.ActiveTimestamp = binary.BigEndian.Uint64(val)
	}
}
