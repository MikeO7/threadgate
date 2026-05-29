package snapshot

import (
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

const testRadioPath = "/dev/ttyUSB0"

func TestBuildSetupGuideHostAndRadio(t *testing.T) {
	audit := hardware.HostAudit{
		IPv6ForwardingAll:     false,
		IPv6ForwardingDefault: false,
		IPv6AcceptRaAll:       false,
		IPv6AcceptRaDefault:   false,
		TunDeviceExists:       true,
	}
	status := runtime.Status{
		ProbeError:     "spinel probe timed out",
		RadioPath:      testRadioPath,
		DetectedDevice: "SONOFF Dongle Plus MG24 (VID: 10c4, PID: ea60)",
	}
	guide := BuildSetupGuide(false, audit, status)
	if !guide.Needed {
		t.Fatal("expected setup guide")
	}
	if guide.Total != 4 {
		t.Fatalf("expected 4 steps, got %d", guide.Total)
	}
	if !strings.Contains(guide.Steps[0].Title, "Step 1:") {
		t.Fatalf("expected numbered title, got %q", guide.Steps[0].Title)
	}
	if !strings.Contains(guide.Steps[2].Description, "Sonoff Dongle Plus MG24") {
		t.Fatalf("expected MG24 flash target, got %q", guide.Steps[2].Description)
	}
	if len(guide.Persist) != 4 {
		t.Fatalf("expected persist snippet lines, got %v", guide.Persist)
	}
}

func TestBuildSetupGuideMockSkips(t *testing.T) {
	guide := BuildSetupGuide(true, hardware.HostAudit{}, runtime.Status{ProbeError: "x"})
	if guide.Needed {
		t.Fatal("mock mode should not show setup guide")
	}
}
