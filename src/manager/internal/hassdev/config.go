// Package hassdev provides Home Assistant bootstrap tooling for ThreadGate integration testing.
package hassdev

import (
	"path/filepath"
	"time"
)

// Config drives local Home Assistant + ThreadGate integration tooling.
type Config struct {
	HAURL      string
	HAPairURL  string // URL saved for ThreadGate → HA (Docker: http://homeassistant:8123)
	TGURL      string
	HAUser     string
	HAPass     string
	HAName     string
	HAClientID   string
	HATimezone   string
	HACountry    string
	HACurrency   string
	HAUnitSystem string
	HALatitude   float64
	HALongitude  float64
	OTBRURL      string

	Root        string
	HAConfigDir string
	CredsFile   string
	FixtureFile string

	HTTPTimeout time.Duration
}

// DefaultConfig returns paths and URLs for the repo integration stack.
func DefaultConfig(root string) Config {
	haURL := envOr("HA_URL", "http://127.0.0.1:8123")
	haConfig := envOr("HA_CONFIG_DIR", root+"/testdata/ha-config")
	creds := envOr("CREDS_FILE", root+"/testdata/ha-credentials.env")
	fixture := envOr("HA_FIXTURE_FILE", root+"/testdata/ha-fixture.tar.gz")
	if v := getenv("HA_CONFIG_DIR"); v != "" {
		root = filepathDir(v)
	}
	return Config{
		HAURL:       haURL,
		HAPairURL:   envOr("HA_PAIR_URL", "http://homeassistant:8123"),
		TGURL:       envOr("TG_URL", "http://127.0.0.1:8081"),
		HAUser:      envOr("HA_USER", "admin"),
		HAPass:      envOr("HA_PASS", "threadgate-test"),
		HAName:      envOr("HA_NAME", "ThreadGate Test"),
		HAClientID:  envOr("HA_CLIENT_ID", haURL+"/"),
		HATimezone:   envOr("HA_TIMEZONE", "America/Los_Angeles"),
		HACountry:    envOr("HA_COUNTRY", "US"),
		HACurrency:   envOr("HA_CURRENCY", "USD"),
		HAUnitSystem: envOr("HA_UNIT_SYSTEM", "us_customary"),
		HALatitude:   envFloatOr("HA_LATITUDE", 39.8283),
		HALongitude:  envFloatOr("HA_LONGITUDE", -98.5795),
		OTBRURL:      envOr("OTBR_URL", "http://threadgate:8081"),
		Root:        root,
		HAConfigDir: haConfig,
		CredsFile:   creds,
		FixtureFile: fixture,
		HTTPTimeout: 30 * time.Second,
	}
}

func envOr(key, fallback string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return fallback
}

func filepathDir(path string) string {
	d := filepath.Dir(path)
	if d == "." || d == "/" {
		return path
	}
	return filepath.Dir(d)
}
