package hass

import (
	"context"
	"encoding/json"
)

type registryDevice struct {
	ID           string  `json:"id"`
	Name         *string `json:"name_by_user"`
	NameDefault  *string `json:"name"`
	Connections  [][]any `json:"connections"`
	Manufacturer *string `json:"manufacturer"`
	Model        *string `json:"model"`
	SwVersion    *string `json:"sw_version"`
}

// ListDevices returns MAC-normalized device details via the device registry WebSocket API.
func ListDevices(ctx context.Context, haURL, accessToken string) (map[string]DeviceDetails, error) {
	c, err := Dial(ctx, haURL, accessToken)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()

	raw, err := c.CallRaw(ctx, "config/device_registry/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var devices []registryDevice
	if err := json.Unmarshal(raw, &devices); err != nil {
		return nil, err
	}
	return mapRegistryDevices(devices), nil
}

func mapRegistryDevices(raw []registryDevice) map[string]DeviceDetails {
	names := make(map[string]DeviceDetails)
	for _, dev := range raw {
		name := registryName(dev)
		if name == "" {
			continue
		}
		manufacturer, model, sw := "", "", ""
		if dev.Manufacturer != nil {
			manufacturer = *dev.Manufacturer
		}
		if dev.Model != nil {
			model = *dev.Model
		}
		if dev.SwVersion != nil {
			sw = *dev.SwVersion
		}
		for mac, details := range deviceFromConnections(name, manufacturer, model, sw, dev.ID, nil, nil, dev.Connections) {
			names[mac] = details
		}
	}
	return names
}

func registryName(dev registryDevice) string {
	if dev.Name != nil && *dev.Name != "" {
		return *dev.Name
	}
	if dev.NameDefault != nil {
		return *dev.NameDefault
	}
	return ""
}
