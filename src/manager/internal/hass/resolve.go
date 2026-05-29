package hass

import (
	"context"
	"fmt"
)

// mapConnection extracts a normalized MAC from a Home Assistant device connection tuple.
func mapConnection(connType, connVal string) (string, bool) {
	if connType != "zigbee" && connType != "mac" && connType != "thread" {
		return "", false
	}
	mac := NormalizeMac(connVal)
	if mac == "" {
		return "", false
	}
	return mac, true
}

func deviceFromConnections(name, manufacturer, model, sw, deviceID string, battery, availability any, connections [][]any) map[string]DeviceDetails {
	names := make(map[string]DeviceDetails)
	for _, conn := range connections {
		if len(conn) < 2 {
			continue
		}
		connType, ok1 := conn[0].(string)
		if !ok1 {
			connType = fmt.Sprint(conn[0])
		}
		connVal := fmt.Sprint(conn[1])
		mac, ok := mapConnection(connType, connVal)
		if !ok {
			continue
		}
		names[mac] = DeviceDetails{
			Name:         name,
			Manufacturer: manufacturer,
			Model:        model,
			SwVersion:    sw,
			Battery:      toString(battery),
			Availability: toString(availability),
			DeviceID:     deviceID,
		}
	}
	return names
}

// mergeDeviceMaps overlays enrichment fields from fallback onto primary registry results.
func mergeDeviceMaps(primary, fallback map[string]DeviceDetails) map[string]DeviceDetails {
	if len(fallback) == 0 {
		return primary
	}
	if primary == nil {
		primary = make(map[string]DeviceDetails)
	}
	for mac, fb := range fallback {
		existing, ok := primary[mac]
		if !ok {
			primary[mac] = fb
			continue
		}
		if existing.Battery == "" {
			existing.Battery = fb.Battery
		}
		if existing.Availability == "" {
			existing.Availability = fb.Availability
		}
		if existing.DeviceID == "" {
			existing.DeviceID = fb.DeviceID
		}
		primary[mac] = existing
	}
	return primary
}

// ResolveDevices fetches device details via registry WebSocket, enriching from template fallback when needed.
func ResolveDevices(ctx context.Context, c *Client, url, token string) (map[string]DeviceDetails, error) {
	names, err := ListDevices(ctx, url, token)
	if err == nil && deviceMapNeedsEnrichment(names) {
		fallback, fbErr := c.fetchViaTemplate(ctx, url, token)
		if fbErr == nil {
			names = mergeDeviceMaps(names, fallback)
		}
		return names, nil
	}
	if err == nil {
		return names, nil
	}
	return c.fetchViaTemplate(ctx, url, token)
}

func deviceMapNeedsEnrichment(names map[string]DeviceDetails) bool {
	for _, dev := range names {
		if dev.Battery != "" || dev.Availability != "" {
			return false
		}
	}
	return len(names) > 0
}
