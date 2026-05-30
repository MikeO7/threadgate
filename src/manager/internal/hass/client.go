// Package hass is the deep Home Assistant client module: WebSocket registry, HTTP fallback, and credential storage.
package hass

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

// Client queries Home Assistant for Thread device friendly names.
type Client struct {
	url        string
	token      string
	httpClient *http.Client
	mockMode   bool

	mu          sync.Mutex
	cache       map[string]DeviceDetails
	cacheExpiry time.Time
	cacheTTL    time.Duration
	lastError   error
}

// NewClient creates a client from orchestrator config and optional saved credentials.
func NewClient(cfg *config.Config) *Client {
	url := cfg.HassURL
	token := cfg.HassToken
	if url == "" || token == "" {
		if fileURL, fileToken := LoadConfig(cfg.StateDir); fileURL != "" && fileToken != "" {
			url = fileURL
			token = fileToken
		}
	}
	return &Client{
		url:      url,
		token:    token,
		mockMode: cfg.Runtime.IsMock(),
		cacheTTL: 5 * time.Minute,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Reload updates credentials and invalidates the device name cache.
func (c *Client) Reload(url, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.url = url
	c.token = token
	c.cache = nil
	c.lastError = nil
}

// Enabled returns true when credentials or mock names are available.
func (c *Client) Enabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hasCredentials() || c.mockMode
}

func (c *Client) hasCredentials() bool {
	return c.url != "" && c.token != ""
}

// Status returns integration status and the last fetch error message.
func (c *Client) Status() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.hasCredentials() {
		if c.mockMode {
			return StatusMock, ""
		}
		return StatusDisabled, ""
	}
	if c.lastError != nil {
		return StatusFailed, c.lastError.Error()
	}
	return StatusConnected, ""
}

// FetchDeviceNames returns MAC-keyed device details from Home Assistant.
func (c *Client) FetchDeviceNames(ctx context.Context) (map[string]DeviceDetails, error) {
	if c.mockMode && !c.hasCredentialsUnlocked() {
		return MockDeviceNames(), nil
	}
	if !c.Enabled() {
		return make(map[string]DeviceDetails), nil
	}

	c.mu.Lock()
	if c.cache != nil && time.Now().Before(c.cacheExpiry) {
		copied := copyDetails(c.cache)
		c.mu.Unlock()
		return copied, nil
	}
	url, token := c.url, c.token
	c.mu.Unlock()

	names, err := resolveDevices(ctx, c, url, token)
	if err != nil {
		c.mu.Lock()
		c.lastError = err
		c.mu.Unlock()
		return nil, err
	}
	c.setCache(names, nil)
	return copyDetails(names), nil
}

func (c *Client) hasCredentialsUnlocked() bool {
	return c.url != "" && c.token != ""
}

func (c *Client) setCache(names map[string]DeviceDetails, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = err
	c.cache = names
	c.cacheExpiry = time.Now().Add(c.cacheTTL)
}

const fetchDevicesTemplate = `{%- set ns = namespace(devices=[]) -%}
{%- for dev_id in device_ids() -%}
  {%- set conns = device_attr(dev_id, 'connections') -%}
  {%- if conns -%}
    {%- set battery = None -%}
    {%- set availability = None -%}
    {%- for entity_id in device_entities(dev_id) -%}
      {%- if 'battery' in entity_id -%}
        {%- set battery = states(entity_id) -%}
      {%- else -%}
        {%- if not availability -%}
          {%- set availability = states(entity_id) -%}
        {%- endif -%}
      {%- endif -%}
    {%- endfor -%}
    {%- if not availability and battery -%}
      {%- set availability = battery -%}
    {%- endif -%}
    {%- set ns.devices = ns.devices + [{
      'name': device_attr(dev_id, 'name_by_user') or device_attr(dev_id, 'name'),
      'connections': conns | list,
      'manufacturer': device_attr(dev_id, 'manufacturer'),
      'model': device_attr(dev_id, 'model'),
      'sw_version': device_attr(dev_id, 'sw_version'),
      'battery': battery,
      'availability': availability,
      'device_id': dev_id
    }] -%}
  {%- endif -%}
{%- endfor -%}
{{ ns.devices | tojson }}`

type templateDevice struct {
	Name         string  `json:"name"`
	Connections  [][]any `json:"connections"`
	Manufacturer string  `json:"manufacturer"`
	Model        string  `json:"model"`
	SwVersion    string  `json:"sw_version"`
	Battery      any     `json:"battery"`
	Availability any     `json:"availability"`
	DeviceID     string  `json:"device_id"`
}

func (c *Client) fetchViaTemplate(ctx context.Context, url, token string) (map[string]DeviceDetails, error) {
	reqBody, err := json.Marshal(map[string]string{"template": fetchDevicesTemplate})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/template", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var devices []templateDevice
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	names := make(map[string]DeviceDetails)
	for _, dev := range devices {
		if dev.Name == "" {
			continue
		}
		for mac, details := range deviceFromConnections(
			dev.Name, dev.Manufacturer, dev.Model, dev.SwVersion, dev.DeviceID,
			dev.Battery, dev.Availability, dev.Connections,
		) {
			names[mac] = details
		}
	}
	return names, nil
}

func toString(val any) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// URL returns the configured Home Assistant base URL or mock default.
func (c *Client) URL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.url == "" && c.mockMode {
		return "http://homeassistant.local:8123"
	}
	return c.url
}

func copyDetails(src map[string]DeviceDetails) map[string]DeviceDetails {
	copied := make(map[string]DeviceDetails, len(src))
	maps.Copy(copied, src)
	return copied
}

// CountDevices returns the number of entries in the HA device registry.
func CountDevices(ctx context.Context, haURL, token string) (int, error) {
	c, err := Dial(ctx, haURL, token)
	if err != nil {
		return 0, err
	}
	defer func() { _ = c.Close() }()
	raw, err := c.CallRaw(ctx, "config/device_registry/list", map[string]any{})
	if err != nil {
		return 0, err
	}
	var devices []json.RawMessage
	if err := json.Unmarshal(raw, &devices); err != nil {
		return 0, err
	}
	return len(devices), nil
}

// Package hass is the deep Home Assistant client module: WebSocket registry, HTTP fallback, and credential storage.

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

	// ConnTypeZigbee is the literal string for a Zigbee connection.
	ConnTypeZigbee = "zigbee"
)

// NormalizeMac strips separators and lowercases a MAC or extended address for map lookup.
func NormalizeMac(mac string) string {
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	return strings.TrimSpace(mac)
}

// LoadConfig reads saved Home Assistant credentials from stateDir.
func LoadConfig(stateDir string) (url, token string) {
	path := configPath(stateDir)
	//nolint:gosec // G304: path is derived from stateDir via configPath, not user-supplied file paths
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var saved SavedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return "", ""
	}
	return saved.HassURL, saved.HassToken
}

// SaveConfig writes Home Assistant credentials to persistent storage.
func SaveConfig(stateDir, url, token string) error {
	path := configPath(stateDir)
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	saved := SavedConfig{HassURL: url, HassToken: token}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func configPath(stateDir string) string {
	if stateDir != "" {
		return filepath.Join(stateDir, "hass_config.json")
	}
	return "/data/hass_config.json"
}

// MockDeviceNames returns simulated friendly names for mock-mode dashboard rendering.
func MockDeviceNames() map[string]DeviceDetails {
	m := map[string]DeviceDetails{
		"0000000000000001": {Name: "Living Room Multi-Sensor", Manufacturer: "Eve", Model: "Eve Motion", SwVersion: "1.2.3", Battery: "82", Availability: "on", DeviceID: "dev_eve_motion"},
		"0000000000000002": {Name: "Kitchen Smart Plug", Manufacturer: "Nanoleaf", Model: "Essentials Smart Plug", SwVersion: "3.1.2", Availability: "on", DeviceID: "dev_nano_plug"},
		"0000000000000003": {Name: "Bedroom Radiator Valve", Manufacturer: "Danfoss", Model: "Ally Radiator Thermostat", SwVersion: "2.1.0", Battery: "12", Availability: "on", DeviceID: "dev_dan_valve"},
		"0000000000000004": {Name: "Hallway Motion Detector", Manufacturer: "Philips", Model: "Hue Motion Sensor", SwVersion: "1.5.0", Battery: "95", Availability: "on", DeviceID: "dev_hue_motion"},
		"0000000000000005": {Name: "Office Desk Lamp", Manufacturer: "Ikea", Model: "Tradfri Bulb", SwVersion: "2.0.0", Availability: "off", DeviceID: "dev_ikea_lamp"},
		"0000000000000006": {Name: "Front Door Lock", Manufacturer: "Yale", Model: "Assure Lock 2", SwVersion: "4.2.1", Battery: "45", Availability: "unavailable", DeviceID: "dev_yale_lock"},
		"1122334455667788": {Name: "ThreadGate Border Router", Manufacturer: "ThreadGate", Model: "Border Router Gateway", SwVersion: "1.0.0", Availability: "on", DeviceID: "dev_threadgate"},
	}
	deviceTypes := []string{"Smart Bulb", "Smart Plug", "Thermostat", "Motion Sensor", "Door Sensor", "Window Shade", "Wall Switch", "Light Dimmer"}
	locations := []string{"Kitchen", "Living Room", "Master Bedroom", "Guest Bedroom", "Office", "Hallway", "Basement", "Garage", "Patio", "Attic"}
	for i := 1; i <= 32; i++ {
		mac := fmt.Sprintf("%016x", i)
		if _, ok := m[mac]; !ok {
			loc := locations[(i-1)%len(locations)]
			dtype := deviceTypes[(i-1)%len(deviceTypes)]
			m[mac] = DeviceDetails{
				Name:         fmt.Sprintf("%s %s", loc, dtype),
				Manufacturer: "Nanoleaf",
				Model:        "Essentials " + dtype,
				SwVersion:    "1.0.1",
				Availability: "on",
				DeviceID:     fmt.Sprintf("dev_mock_router_%d", i),
			}
		}
	}
	for i := range 12 {
		mac := fmt.Sprintf("e00000000000%04x", i)
		if _, ok := m[mac]; !ok {
			loc := locations[i%len(locations)]
			m[mac] = DeviceDetails{
				Name:         fmt.Sprintf("%s Sleepy Sensor %d", loc, i+1),
				Manufacturer: "Eve",
				Model:        "Eve Door & Window",
				SwVersion:    "2.0.2",
				Battery:      fmt.Sprintf("%d", (i*8)%100+10),
				Availability: "on",
				DeviceID:     fmt.Sprintf("dev_mock_sleepy_%d", i),
			}
		}
	}
	return m
}

type registryDevice struct {
	ID           string  `json:"id"`
	Name         *string `json:"name_by_user"`
	NameDefault  *string `json:"name"`
	Connections  [][]any `json:"connections"`
	Manufacturer *string `json:"manufacturer"`
	Model        *string `json:"model"`
	SwVersion    *string `json:"sw_version"`
}

// listDevices returns MAC-normalized device details via the device registry WebSocket API.
func listDevices(ctx context.Context, haURL, accessToken string) (map[string]DeviceDetails, error) {
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

// mapConnection extracts a normalized MAC from a Home Assistant device connection tuple.
func mapConnection(connType, connVal string) (string, bool) {
	if connType != ConnTypeZigbee && connType != "mac" && connType != "thread" {
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

// resolveDevices fetches device details via registry WebSocket, enriching from template fallback when needed.
func resolveDevices(ctx context.Context, c *Client, url, token string) (map[string]DeviceDetails, error) {
	names, err := listDevices(ctx, url, token)
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
