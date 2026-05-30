package hassdev

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MockThreadDevice matches ThreadGate mock topology MACs (see internal/hass/mock.go).
type MockThreadDevice struct {
	Name         string
	MAC          string
	Manufacturer string
	Model        string
}

// CoreMockDevices are seeded into Home Assistant for integration testing.
var CoreMockDevices = []MockThreadDevice{
	{Name: "Living Room Multi-Sensor", MAC: "0000000000000001", Manufacturer: "Eve", Model: "Eve Motion"},
	{Name: "Kitchen Smart Plug", MAC: "0000000000000002", Manufacturer: "Nanoleaf", Model: "Essentials Smart Plug"},
	{Name: "Bedroom Radiator Valve", MAC: "0000000000000003", Manufacturer: "Danfoss", Model: "Ally Radiator Thermostat"},
	{Name: "Hallway Motion Detector", MAC: "0000000000000004", Manufacturer: "Philips", Model: "Hue Motion Sensor"},
	{Name: "Office Desk Lamp", MAC: "0000000000000005", Manufacturer: "Ikea", Model: "Tradfri Bulb"},
	{Name: "ThreadGate Border Router", MAC: "1122334455667788", Manufacturer: "ThreadGate", Model: "Border Router Gateway"},
}

// SeedDeviceRegistry writes mock Thread devices into HA .storage (HA must be stopped).
func SeedDeviceRegistry(configDir string) (int, error) {
	path := filepath.Join(configDir, ".storage", "core.device_registry")
	store, err := loadDeviceRegistry(path)
	if err != nil {
		return 0, err
	}

	known := knownDeviceMACs(store.Data.Devices)
	created := appendMissingMockDevices(&store, known)
	if err := writeDeviceRegistry(path, store); err != nil {
		return 0, err
	}
	return created, nil
}

func loadDeviceRegistry(path string) (deviceRegistryStore, error) {
	store := deviceRegistryStore{
		Version:      1,
		MinorVersion: 12,
		Key:          "core.device_registry",
		Data: deviceRegistryData{
			Devices:        []deviceRegistryEntry{},
			DeletedDevices: []any{},
		},
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path confined to HA .storage/core.device_registry
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return deviceRegistryStore{}, err
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return deviceRegistryStore{}, fmt.Errorf("parse device registry: %w", err)
	}
	return store, nil
}

func knownDeviceMACs(devices []deviceRegistryEntry) map[string]struct{} {
	known := map[string]struct{}{}
	for _, dev := range devices {
		for _, conn := range dev.Connections {
			if len(conn) >= 2 {
				known[normalizeMAC(fmt.Sprint(conn[1]))] = struct{}{}
			}
		}
	}
	return known
}

func appendMissingMockDevices(store *deviceRegistryStore, known map[string]struct{}) int {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	created := 0
	for _, spec := range CoreMockDevices {
		mac := normalizeMAC(spec.MAC)
		if _, ok := known[mac]; ok {
			continue
		}
		store.Data.Devices = append(store.Data.Devices, newMockDeviceEntry(spec, now))
		created++
	}
	return created
}

func newMockDeviceEntry(spec MockThreadDevice, now string) deviceRegistryEntry {
	return deviceRegistryEntry{
		AreaID:                  nil,
		ConfigEntries:           []string{},
		ConfigEntriesSubentries: map[string][]any{},
		ConfigurationURL:        nil,
		Connections:             [][]string{{"thread", spec.MAC}},
		CreatedAt:               now,
		DisabledBy:              nil,
		EntryType:               nil,
		HWVersion:               nil,
		ID:                      randomHexID(),
		Identifiers:             [][]string{},
		Labels:                  []string{},
		Manufacturer:            spec.Manufacturer,
		Model:                   spec.Model,
		ModelID:                 nil,
		ModifiedAt:              now,
		NameByUser:              spec.Name,
		Name:                    spec.Name,
		PrimaryConfigEntry:      nil,
		SerialNumber:            nil,
		SwVersion:               "1.0.0",
		ViaDeviceID:             nil,
	}
}

func writeDeviceRegistry(path string, store deviceRegistryStore) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return err
	}
	out, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, filePerm)
}

func normalizeMAC(mac string) string {
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	return strings.TrimSpace(mac)
}

func randomHexID() string {
	b := make([]byte, 16)
	_, _ = randRead(b)
	return fmt.Sprintf("%x", b)
}

type deviceRegistryStore struct {
	Version      int                `json:"version"`
	MinorVersion int                `json:"minor_version"`
	Key          string             `json:"key"`
	Data         deviceRegistryData `json:"data"`
}

type deviceRegistryData struct {
	Devices        []deviceRegistryEntry `json:"devices"`
	DeletedDevices []any                 `json:"deleted_devices"`
}

type deviceRegistryEntry struct {
	AreaID                  any              `json:"area_id"`
	ConfigEntries           []string         `json:"config_entries"`
	ConfigEntriesSubentries map[string][]any `json:"config_entries_subentries"`
	ConfigurationURL        any              `json:"configuration_url"`
	Connections             [][]string       `json:"connections"`
	CreatedAt               string           `json:"created_at"`
	DisabledBy              any              `json:"disabled_by"`
	EntryType               any              `json:"entry_type"`
	HWVersion               any              `json:"hw_version"`
	ID                      string           `json:"id"`
	Identifiers             [][]string       `json:"identifiers"`
	Labels                  []string         `json:"labels"`
	Manufacturer            string           `json:"manufacturer"`
	Model                   string           `json:"model"`
	ModelID                 any              `json:"model_id"`
	ModifiedAt              string           `json:"modified_at"`
	NameByUser              string           `json:"name_by_user"`
	Name                    string           `json:"name"`
	PrimaryConfigEntry      any              `json:"primary_config_entry"`
	SerialNumber            any              `json:"serial_number"`
	SwVersion               string           `json:"sw_version"`
	ViaDeviceID             any              `json:"via_device_id"`
}
