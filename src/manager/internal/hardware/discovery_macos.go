package hardware

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// runIoregCmd executes the ioreg command on macOS to fetch USB device trees. (Overridable in tests).
var runIoregCmd = func() ([]byte, error) {
	cmd := exec.Command("ioreg", "-p", "IOUSB", "-l")
	return cmd.Output()
}

type ioregDeviceBlock struct {
	product string
	vendor  string
	vid     string
	pid     string
}

var macCoordinatorNameHints = []string{
	hwNameSonoff, "dongle", hwNameSkyConnect, hwNameZBT, hwNameCP210, hwNameCH34,
	hwNameFTDI, hwNamePL2303, "nordic", hwNameNRF52840,
}

var macCoordinatorVendorHints = []string{
	hwNameSonoff, hwNameSilabs, "nordic",
}

func parseIORegQuotedValue(line string) string {
	parts := strings.Split(line, "=")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Trim(parts[1], " \"")
}

func applyIORegLine(line string, block *ioregDeviceBlock) {
	switch {
	case strings.Contains(line, `"USB Product Name"`), strings.Contains(line, `"kUSBProductString"`):
		block.product = parseIORegQuotedValue(line)
	case strings.Contains(line, `"USB Vendor Name"`), strings.Contains(line, `"kUSBVendorString"`):
		block.vendor = parseIORegQuotedValue(line)
	case strings.Contains(line, `"idVendor"`):
		block.vid = strings.TrimSpace(parseIORegQuotedValue(line))
	case strings.Contains(line, `"idProduct"`):
		block.pid = strings.TrimSpace(parseIORegQuotedValue(line))
	}
}

func parseDecimalUSBIDs(vid, pid string) (int64, int64, bool) {
	if vid == "" || pid == "" {
		return 0, 0, false
	}
	vDec, err := strconv.ParseInt(vid, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	pDec, err := strconv.ParseInt(pid, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return vDec, pDec, true
}

func containsAnyHint(value string, hints []string) bool {
	for _, hint := range hints {
		if strings.Contains(value, hint) {
			return true
		}
	}
	return false
}

func isKnownMacCoordinatorName(product, vendor string) bool {
	lowerProd := strings.ToLower(product)
	lowerVendor := strings.ToLower(vendor)
	return containsAnyHint(lowerProd, macCoordinatorNameHints) ||
		containsAnyHint(lowerVendor, macCoordinatorVendorHints)
}

func macCoordinatorDescription(product, vendor, signatureDesc string) string {
	if signatureDesc != "" {
		if product != "" {
			return fmt.Sprintf("%s (%s)", product, signatureDesc)
		}
		return signatureDesc
	}
	if vendor != "" && !strings.Contains(strings.ToLower(product), strings.ToLower(vendor)) {
		return fmt.Sprintf("%s %s", vendor, product)
	}
	return product
}

func matchMacCoordinatorBlock(block ioregDeviceBlock) (desc, vid, pid string, found bool) {
	vDec, pDec, ok := parseDecimalUSBIDs(block.vid, block.pid)
	if !ok {
		return "", "", "", false
	}

	hexVID := fmt.Sprintf("%04x", vDec)
	hexPID := fmt.Sprintf("%04x", pDec)
	hexKey := hexVID + ":" + hexPID
	if sig, exists := targetSignatures[hexKey]; exists {
		return macCoordinatorDescription(block.product, block.vendor, sig.Desc), hexVID, hexPID, true
	}
	if !isKnownMacCoordinatorName(block.product, block.vendor) {
		return "", "", "", false
	}
	return macCoordinatorDescription(block.product, block.vendor, ""), hexVID, hexPID, true
}

// coordinatorSettingsFromMacIOReg maps a connected macOS USB coordinator to recommended serial settings.
func coordinatorSettingsFromMacIOReg() (baud int, flow bool, ok bool) {
	_, vid, pid, found := DetectMacSerialSignature()
	if !found {
		return 0, false, false
	}
	sig, exists := targetSignatures[vid+":"+pid]
	if !exists {
		return 0, false, false
	}
	return sig.Baudrate, sig.FlowControl, true
}

// DetectMacSerialSignature scans macOS USB devices to see if a known coordinator is plugged in.
func DetectMacSerialSignature() (desc, vid, pid string, found bool) {
	out, err := runIoregCmd()
	if err != nil {
		return "", "", "", false
	}

	var block ioregDeviceBlock
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, `"idProduct"`) {
			applyIORegLine(trimmed, &block)
			continue
		}

		applyIORegLine(trimmed, &block)
		if desc, vid, pid, found = matchMacCoordinatorBlock(block); found {
			return desc, vid, pid, true
		}
		block = ioregDeviceBlock{}
	}
	return "", "", "", false
}
