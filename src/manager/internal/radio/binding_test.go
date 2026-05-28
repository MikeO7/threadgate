package radio

import (
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

func TestBindingMockRefresh(t *testing.T) {
	tracker := runtime.NewTracker()
	cfg := Config{
		AutoDiscover: true,
		Baudrate:     460800,
		MockMode:     true,
	}
	b, err := NewBinding(cfg, tracker)
	if err != nil {
		t.Fatalf("NewBinding: %v", err)
	}
	if b.CurrentSpinelURL() != "spinel+hdlc+uart:///dev/ttyMOCK0?uart-baudrate=460800" {
		t.Errorf("unexpected URL: %q", b.CurrentSpinelURL())
	}
	if err := b.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	status := tracker.GetStatus()
	if status.RadioPath != "/dev/ttyMOCK0" {
		t.Errorf("expected radio path /dev/ttyMOCK0, got %q", status.RadioPath)
	}
	if status.ProbedVersion == "" {
		t.Error("expected mock probed version after refresh")
	}
}
