package hass

import "strings"

// NormalizeMac strips separators and lowercases a MAC or extended address for map lookup.
func NormalizeMac(mac string) string {
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	return strings.TrimSpace(mac)
}
