package snapshot

import (
	"strconv"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

const silabsFlasherURL = "https://darkxst.github.io/silabs-firmware-builder/"

// SetupGuide is the ordered production checklist shown on the dashboard.
type SetupGuide struct {
	Needed   bool
	Preview  bool
	Complete int
	Total    int
	Steps    []hardware.SetupStep
	Persist  []string
}

// BuildSetupGuide assembles host and radio setup steps for the dashboard.
func BuildSetupGuide(mockRadio, mockSetupChecklist bool, audit hardware.HostAudit, status runtime.Status) SetupGuide {
	if mockSetupChecklist {
		return mockSetupGuidePreview()
	}
	if mockRadio {
		return SetupGuide{}
	}

	return buildSetupGuideFromState(audit, status, false)
}

func buildSetupGuideFromState(audit hardware.HostAudit, status runtime.Status, preview bool) SetupGuide {
	steps := hardware.HostNetworkingSteps(audit)
	steps = append(steps, radioSetupSteps(status)...)

	for i := range steps {
		steps[i].Title = formatStepTitle(i+1, steps[i].Title)
	}

	complete := 0
	for _, step := range steps {
		if step.Done {
			complete++
		}
	}

	return SetupGuide{
		Needed:   len(steps) > 0,
		Preview:  preview,
		Complete: complete,
		Total:    len(steps),
		Steps:    steps,
		Persist:  hardware.PersistSysctlSnippet(audit),
	}
}

const mockPreviewRadioPath = "/dev/ttyUSB0"

func mockSetupGuidePreview() SetupGuide {
	audit := hardware.HostAudit{TunDeviceExists: true}
	status := runtime.Status{
		ProbeError:     "preview: spinel probe timed out or returned invalid response",
		RadioPath:      mockPreviewRadioPath,
		DetectedDevice: "SONOFF Dongle Plus MG24 (VID: 10c4, PID: ea60)",
	}
	return buildSetupGuideFromState(audit, status, true)
}

func formatStepTitle(number int, title string) string {
	return strings.TrimSpace(strings.Join([]string{"Step", strconv.Itoa(number) + ":", title}, " "))
}

func radioSetupSteps(status runtime.Status) []hardware.SetupStep {
	if status.ProbeError == "" && status.ProbedVersion != "" {
		return nil
	}
	if status.ProbeError == "" {
		return nil
	}

	device := strings.ToLower(status.DetectedDevice)
	flasherDevice, baud := flashTargetForDevice(device)

	steps := []hardware.SetupStep{
		{
			ID:          "radio-flash-rcp",
			Title:       "Flash OpenThread RCP firmware onto the USB dongle",
			Description: flashDescription(device, flasherDevice, baud),
			Done:        false,
			Note:        "Web flasher: " + silabsFlasherURL,
		},
		{
			ID:          "radio-recreate-container",
			Title:       "Restart ThreadGate after flashing",
			Description: "Unplug and replug the dongle, then recreate the container so the border router picks up the radio cleanly.",
			Done:        false,
			Commands: []string{
				"docker compose -f docker-compose.test-server.yml up -d --force-recreate threadgate",
			},
			Note: "Use docker-compose.yml instead if that is what you deployed with.",
		},
	}
	return steps
}

func flashTargetForDevice(deviceLower string) (target, baud string) {
	switch {
	case strings.Contains(deviceLower, "mg24"):
		return "Sonoff Dongle Plus MG24", "460800"
	case strings.Contains(deviceLower, "sonoff"):
		return "Sonoff ZBDongle-E", "460800"
	case strings.Contains(deviceLower, "skyconnect"), strings.Contains(deviceLower, "zbt"):
		return "Home Assistant SkyConnect / Connect ZBT-1", "460800"
	case strings.Contains(deviceLower, "nordic"), strings.Contains(deviceLower, "nrf52840"):
		return "Nordic nRF52840 Thread RCP", "115200"
	default:
		return "your coordinator model", "460800"
	}
}

func flashDescription(deviceLower, flasherDevice, baud string) string {
	if deviceLower == "" {
		return "ThreadGate could not talk to the radio over Spinel. Flash standard OpenThread RCP firmware using the Silicon Labs web flasher."
	}
	return "Detected " + strings.TrimSpace(statusDeviceName(deviceLower)) +
		", but it is not responding with Thread RCP firmware. In the web flasher select " + flasherDevice +
		", choose OpenThread RCP (Thread), and flash at " + baud + " baud with hardware flow control if offered."
}

func statusDeviceName(deviceLower string) string {
	if idx := strings.Index(deviceLower, "(vid:"); idx > 0 {
		return strings.TrimSpace(deviceLower[:idx])
	}
	return deviceLower
}
