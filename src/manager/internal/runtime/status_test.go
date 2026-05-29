package runtime

import (
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
)

func TestTracker(t *testing.T) {
	tracker := NewTracker()
	tracker.SetHostAudit(hardware.HostAudit{TunDeviceExists: true})
	tracker.UpdateRadioHealth("/dev/ttyTEST", "TestVersion/1.0", "", "")
	tracker.UpdateAgent("running", "")

	status := tracker.GetStatus()
	if status.ProbedVersion != "TestVersion/1.0" {
		t.Errorf("expected probed version, got %q", status.ProbedVersion)
	}
	if status.RadioPath != "/dev/ttyTEST" {
		t.Errorf("expected radio path, got %q", status.RadioPath)
	}
	if status.Agent.State != "running" {
		t.Errorf("expected agent running, got %q", status.Agent.State)
	}

	tracker.UpdateRadioHealth("/dev/ttyUPDATED", "NewVersion/2.0", "probe failed", "")
	status = tracker.GetStatus()
	if status.RadioPath != "/dev/ttyUPDATED" {
		t.Errorf("expected updated path, got %q", status.RadioPath)
	}
	if status.ProbeError != "probe failed" {
		t.Errorf("expected probe error, got %q", status.ProbeError)
	}
}
