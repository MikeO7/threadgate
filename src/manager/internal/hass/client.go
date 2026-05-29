package hass

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
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

	names, err := ResolveDevices(ctx, c, url, token)
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
