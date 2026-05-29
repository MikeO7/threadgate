package hardware

import "testing"

func TestHostNetworkingStepsIPv6(t *testing.T) {
	audit := HostAudit{
		IPv6ForwardingAll:     false,
		IPv6ForwardingDefault: false,
		IPv6AcceptRaAll:       false,
		IPv6AcceptRaDefault:   false,
		TunDeviceExists:       true,
	}
	steps := HostNetworkingSteps(audit)
	if len(steps) != 2 {
		t.Fatalf("expected 2 host steps, got %d", len(steps))
	}
	if steps[0].ID != hostStepIPv6Forwarding {
		t.Fatalf("unexpected first step: %q", steps[0].ID)
	}
	if len(steps[0].Commands) != 2 {
		t.Fatalf("expected forwarding commands, got %v", steps[0].Commands)
	}
	if steps[1].ID != hostStepIPv6AcceptRA {
		t.Fatalf("unexpected second step: %q", steps[1].ID)
	}
}

func TestPersistSysctlSnippet(t *testing.T) {
	audit := HostAudit{
		IPv6ForwardingAll:   false,
		IPv6AcceptRaDefault: false,
		TunDeviceExists:     true,
	}
	lines := PersistSysctlSnippet(audit)
	if len(lines) != 4 {
		t.Fatalf("expected 4 persist lines, got %v", lines)
	}
}
