// Package config manages system-wide environment variables and safe operational defaults.
package config

import (
	"net"
	"os"
	"strconv"
	"strings"
)

// Config holds all orchestrator configuration parameters
type Config struct {
	RadioURL            string // Spinel URL e.g. "spinel+hdlc+uart:///dev/ttyUSB0"
	Baudrate            int    // UART Baudrate, e.g. 460800
	LogLevel            string // "debug", "info", "warn", "error"
	Port                int    // Web / REST API port, defaults to 8081
	AutoDiscover        bool   // Whether to auto-discover USB serial devices
	StateDir            string // Directory to persist operational state, defaults to /data
	Runtime             RuntimeMode
	FlowControl         bool   // Whether hardware flow control is enabled (e.g. for SkyConnect)
	ExplicitFlowControl bool   // Whether FlowControl was set explicitly in the environment
	ExplicitBaudrate    bool   // Whether Baudrate was set explicitly in the environment
	BackboneIF          string // Backbone interface for border routing (e.g. eth0, wlan0)
	HassURL             string // Home Assistant URL e.g. "http://192.168.1.100:8123"
	HassToken           string // Home Assistant Long-Lived Access Token
}

const defaultBackboneInterface = "eth0"

// Load reads values from the environment or assigns safe defaults
func Load() *Config {
	mockMode := getEnvBool("OTBR_MOCK_MODE", false)
	_, explicitFlowControl := os.LookupEnv("OTBR_FLOW_CONTROL")
	_, explicitBaudrate := os.LookupEnv("OTBR_BAUDRATE")
	cfg := &Config{
		RadioURL:            os.Getenv("OTBR_RADIO_URL"),
		LogLevel:            getEnv("OTBR_LOG_LEVEL", "info"),
		AutoDiscover:        getEnvBool("OTBR_AUTO_DISCOVER", true),
		StateDir:            getEnv("OTBR_STATE_DIR", "/data"),
		Runtime:             RuntimeModeFromMock(mockMode),
		FlowControl:         getEnvBool("OTBR_FLOW_CONTROL", false),
		ExplicitFlowControl: explicitFlowControl,
		ExplicitBaudrate:    explicitBaudrate,
		HassURL:             strings.TrimSuffix(os.Getenv("OTBR_HASS_URL"), "/"),
		HassToken:           os.Getenv("OTBR_HASS_TOKEN"),
	}

	baud, err := strconv.Atoi(os.Getenv("OTBR_BAUDRATE"))
	if err != nil || baud <= 0 {
		cfg.Baudrate = 460800 // Default optimized baudrate for smart home Thread devices
	} else {
		cfg.Baudrate = baud
	}

	port, err := strconv.Atoi(os.Getenv("OTBR_PORT"))
	if err != nil || port <= 0 {
		cfg.Port = 8081 // standard Home Assistant OTBR REST API port
	} else {
		cfg.Port = port
	}

	backbone := os.Getenv("OTBR_BACKBONE_IF")
	if backbone == "" {
		backbone = detectBackboneInterface()
	}
	cfg.BackboneIF = backbone

	return cfg
}

var listInterfaces = net.Interfaces

func detectBackboneInterface() string {
	ifaces, err := listInterfaces()
	if err != nil {
		return defaultBackboneInterface
	}
	for _, iface := range ifaces {
		// Skip loopback, down interfaces, or virtual mesh interface
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 || iface.Name == "wpan0" {
			continue
		}
		// Skip typical docker-internal bridges
		if strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "br-") || strings.HasPrefix(iface.Name, "veth") || strings.HasPrefix(iface.Name, "lo") {
			continue
		}
		return iface.Name
	}
	return defaultBackboneInterface
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return defaultVal
}
