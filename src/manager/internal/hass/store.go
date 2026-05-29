package hass

import (
	"encoding/json"
	"os"
	"path/filepath"
)

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
