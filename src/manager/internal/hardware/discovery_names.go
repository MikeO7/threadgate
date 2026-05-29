package hardware

import "strings"

const (
	hwNameZBT        = "zbt"
	hwNameSkyConnect = "skyconnect"
	hwNameSonoff     = "sonoff"
	hwNameOpenThread = "openthread"
	hwNameNRF52840   = "nrf52840"
	hwNameUSBSerial  = "usb-serial"
	hwNameCP210      = "cp210"
	hwNameCH34       = "ch34"
	hwNameFTDI       = "ftdi"
	hwNamePL2303     = "pl2303"
	hwNameProlific   = "prolific"
	hwNameSilabs     = "silabs"
)

var (
	knownHardwareSubstrings = []string{
		hwNameZBT, hwNameSkyConnect, hwNameSonoff, hwNameOpenThread, hwNameNRF52840,
		hwNameUSBSerial, hwNameCP210, hwNameCH34, hwNameFTDI, hwNamePL2303, hwNameProlific,
	}
	highSpeedHardwareSubstrings = []string{
		hwNameZBT, hwNameSkyConnect, hwNameSonoff, hwNameCP210,
	}
	standardSpeedHardwareSubstrings = []string{
		hwNameNRF52840, hwNameOpenThread, hwNameCH34, hwNameFTDI, hwNamePL2303, hwNameProlific,
	}
	flowControlHardwareSubstrings = []string{
		hwNameZBT, hwNameSkyConnect, hwNameSonoff, hwNameSilabs, hwNameCP210,
	}
)

// isKnownHardwareName performs signature checks on file names for typical Thread modules.
func isKnownHardwareName(name string) bool {
	name = strings.ToLower(name)
	for _, sub := range knownHardwareSubstrings {
		if strings.Contains(name, sub) {
			return true
		}
	}
	return false
}

// getBaudrateFromHardwareName maps typical smart home USB dongle names to their recommended baud rates.
func getBaudrateFromHardwareName(name string) int {
	name = strings.ToLower(name)
	for _, sub := range highSpeedHardwareSubstrings {
		if strings.Contains(name, sub) {
			return 460800
		}
	}
	for _, sub := range standardSpeedHardwareSubstrings {
		if strings.Contains(name, sub) {
			return 115200
		}
	}
	return 0
}

// getFlowControlFromHardwareName maps typical smart home USB dongle names to their recommended hardware flow control setting.
func getFlowControlFromHardwareName(name string) bool {
	name = strings.ToLower(name)
	for _, sub := range flowControlHardwareSubstrings {
		if strings.Contains(name, sub) {
			return true
		}
	}
	return false
}
