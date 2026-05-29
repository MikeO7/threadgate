package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

// HassClient handles queries to Home Assistant REST API.
type HassClient struct {
	url        string
	token      string
	httpClient *http.Client
	mockMode   bool

	mu          sync.Mutex
	cache       map[string]string
	cacheExpiry time.Time
	cacheTTL    time.Duration
}

// NewHassClient creates a client configured with Home Assistant coordinates.
func NewHassClient(cfg *config.Config) *HassClient {
	return &HassClient{
		url:      cfg.HassURL,
		token:    cfg.HassToken,
		mockMode: cfg.Runtime.IsMock(),
		cacheTTL: 5 * time.Minute,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enabled returns true if HASS integration is configured.
func (c *HassClient) Enabled() bool {
	return (c.url != "" && c.token != "") || c.mockMode
}

type hassDevice struct {
	Name        string          `json:"name"`
	Connections [][]interface{} `json:"connections"`
}

// FetchDeviceNames queries Home Assistant for Thread device friendly names.
func (c *HassClient) FetchDeviceNames(ctx context.Context) (map[string]string, error) {
	if c.mockMode && (c.url == "" || c.token == "") {
		m := map[string]string{
			"0000000000000001": "Living Room Multi-Sensor",
			"0000000000000002": "Kitchen Smart Plug",
			"0000000000000003": "Bedroom Radiator Valve",
			"0000000000000004": "Hallway Motion Detector",
			"0000000000000005": "Office Desk Lamp",
			"0000000000000006": "Front Door Lock",
			"1122334455667788": "ThreadGate Border Router",
		}
		// Dynamically generate names for all other potential mock routers (up to 32)
		deviceTypes := []string{"Smart Bulb", "Smart Plug", "Thermostat", "Motion Sensor", "Door Sensor", "Window Shade", "Wall Switch", "Light Dimmer"}
		locations := []string{"Kitchen", "Living Room", "Master Bedroom", "Guest Bedroom", "Office", "Hallway", "Basement", "Garage", "Patio", "Attic"}
		for i := 1; i <= 32; i++ {
			mac := fmt.Sprintf("%016x", i)
			if _, ok := m[mac]; !ok {
				loc := locations[(i-1)%len(locations)]
				dtype := deviceTypes[(i-1)%len(deviceTypes)]
				m[mac] = fmt.Sprintf("%s %s", loc, dtype)
			}
		}
		// Dynamically generate names for sleepy end devices (up to 12)
		for i := 0; i < 12; i++ {
			mac := fmt.Sprintf("e00000000000%04x", i)
			if _, ok := m[mac]; !ok {
				loc := locations[i%len(locations)]
				m[mac] = fmt.Sprintf("%s Sleepy Sensor %d", loc, i+1)
			}
		}
		return m, nil
	}

	if !c.Enabled() {
		return nil, nil
	}

	c.mu.Lock()
	if c.cache != nil && time.Now().Before(c.cacheExpiry) {
		copied := make(map[string]string, len(c.cache))
		for k, v := range c.cache {
			copied[k] = v
		}
		c.mu.Unlock()
		return copied, nil
	}
	c.mu.Unlock()

	// Build Jinja2 template request to extract devices
	const templateStr = `{%- set ns = namespace(devices=[]) -%}
{%- for dev_id in device_ids() -%}
  {%- set conns = device_attr(dev_id, 'connections') -%}
  {%- if conns -%}
    {%- set name = device_attr(dev_id, 'name_by_user') or device_attr(dev_id, 'name') -%}
    {%- set ns.devices = ns.devices + [{'name': name, 'connections': conns | list}] -%}
  {%- endif -%}
{%- endfor -%}
{{ ns.devices | tojson }}`

	reqBody, err := json.Marshal(map[string]string{"template": templateStr})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/api/template", c.url)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var devices []hassDevice
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	names := make(map[string]string)
	for _, dev := range devices {
		if dev.Name == "" {
			continue
		}
		for _, conn := range dev.Connections {
			if len(conn) < 2 {
				continue
			}
			connType, ok1 := conn[0].(string)
			connVal, ok2 := conn[1].(string)
			if !ok1 || !ok2 {
				continue
			}
			if connType == "zigbee" || connType == "mac" || connType == "thread" {
				normalized := normalizeMac(connVal)
				if normalized != "" {
					names[normalized] = dev.Name
				}
			}
		}
	}

	c.mu.Lock()
	c.cache = names
	c.cacheExpiry = time.Now().Add(c.cacheTTL)
	copied := make(map[string]string, len(c.cache))
	for k, v := range c.cache {
		copied[k] = v
	}
	c.mu.Unlock()

	return copied, nil
}

func normalizeMac(mac string) string {
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	return strings.TrimSpace(mac)
}
