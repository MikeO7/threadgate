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
	guide := BuildSetupGuide(false, false, audit, status)
	if !guide.Needed {
		t.Fatal("expected setup guide")
	}
	if guide.Preview {
		t.Fatal("expected live guide, not preview")
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

func TestBuildSetupGuideMockRadioSkips(t *testing.T) {
	guide := BuildSetupGuide(true, false, hardware.HostAudit{}, runtime.Status{ProbeError: "x"})
	if guide.Needed {
		t.Fatal("full mock radio mode should not show setup guide")
	}
}

func TestBuildSetupGuideMockPreview(t *testing.T) {
	guide := BuildSetupGuide(false, true, hardware.HostAudit{}, runtime.Status{})
	if !guide.Needed {
		t.Fatal("expected preview setup guide")
	}
	if !guide.Preview {
		t.Fatal("expected preview flag")
	}
	if guide.Total != 4 {
		t.Fatalf("expected 4 preview steps, got %d", guide.Total)
	}
	if len(guide.Persist) != 4 {
		t.Fatalf("expected persist snippet in preview, got %v", guide.Persist)
	}
}

func TestBuildSetupGuideMockPreviewOverridesMockRadio(t *testing.T) {
	guide := BuildSetupGuide(true, true, hardware.HostAudit{}, runtime.Status{})
	if !guide.Needed || !guide.Preview {
		t.Fatal("preview flag should win over full mock radio mode")
	}
}
