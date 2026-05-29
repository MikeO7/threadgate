// Package hass is the deep Home Assistant client module: WebSocket registry, HTTP fallback, and credential storage.
package hass

// DeviceDetails is friendly metadata for a Thread/Zigbee device keyed by normalized MAC.
type DeviceDetails struct {
	Name         string
	Manufacturer string
	Model        string
	SwVersion    string
	Battery      string
	Availability string
	DeviceID     string
}

// SavedConfig is persisted after pairing approval.
type SavedConfig struct {
	HassURL   string `json:"hass_url"`
	HassToken string `json:"hass_token"`
}

const (
	StatusConnected = "Connected"
	StatusFailed    = "Failed"
	StatusDisabled  = "Disabled"
	StatusPending   = "Pending"
	StatusMock      = "Mock"
)
