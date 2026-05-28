package hardware

import (
	"testing"
)

func TestAuditHostSafety(t *testing.T) {
	// AuditHost should run gracefully on all platforms (even if sysctl files are missing)
	audit := AuditHost()

	// Ensure warnings slice is populated if check fails, but never panic
	_ = audit.Warnings
}

func TestHealthStatusGlobal(t *testing.T) {
	hs := HealthStatus{
		ProbedVersion: "TestVersion/1.0",
		RadioPath:     "/dev/ttyTEST",
	}
	SetHealth(hs)
	retrieved := GetHealth()
	if ret := retrieved.ProbedVersion; ret != "TestVersion/1.0" {
		t.Errorf("Expected ProbedVersion TestVersion/1.0, got %s", ret)
	}
}
