package snapshot

import (
	"strings"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

func TestRadioBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   runtime.Status
		mockMode bool
		label    string
		variant  string
	}{
		{
			name:     "mock mode",
			mockMode: true,
			label:    "Simulated Radio",
			variant:  badgeVariantWarning,
		},
		{
			name: "probe error",
			status: runtime.Status{
				RadioPath:  "/dev/ttyUSB0",
				ProbeError: "timeout",
			},
			label:   "Dongle Error",
			variant: badgeVariantDanger,
		},
		{
			name: "connected",
			status: runtime.Status{
				RadioPath:     "/dev/ttyUSB0",
				ProbedVersion: "RCP/2.0",
				Agent:         runtime.AgentStatus{State: "running"},
			},
			label:   "Radio Connected",
			variant: badgeVariantSuccess,
		},
		{
			name: "waiting for hardware",
			status: runtime.Status{
				Agent: runtime.AgentStatus{State: "waiting_for_hardware"},
			},
			label:   "Waiting for Dongle",
			variant: badgeVariantWarning,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertStatusBadge(t, RadioBadge(tc.status, tc.mockMode), tc.label, tc.variant)
		})
	}
}

func TestHassBadge(t *testing.T) {
	t.Parallel()

	if got := HassBadge(hass.StatusConnected, ""); got.Label != "HASS Connected" || got.Variant != badgeVariantCyan {
		t.Fatalf("connected badge = %+v", got)
	}
	if got := HassBadge(hass.StatusFailed, "connection refused"); got.Label != "HASS Failed" || got.Variant != badgeVariantDanger {
		t.Fatalf("failed badge = %+v", got)
	}
	if got := HassBadge(hass.StatusDisabled, ""); got.Label != "HASS Not Configured" || got.Variant != badgeVariantMuted {
		t.Fatalf("disabled badge = %+v", got)
	}
}

func TestThreadBadge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		snap    topology.Snapshot
		label   string
		variant string
	}{
		{
			name: "leader with devices",
			snap: topology.Snapshot{
				State:       threadStateLeader,
				NetworkName: "Home",
				Neighbors:   []topology.Neighbor{{}, {}},
				LeaderData:  topology.LeaderData{PartitionID: 123},
			},
			label:   "Thread Leader",
			variant: badgeVariantSuccess,
		},
		{
			name: "idle leader",
			snap: topology.Snapshot{
				State:       threadStateLeader,
				NetworkName: "Home",
			},
			label:   "Idle Leader",
			variant: badgeVariantWarning,
		},
		{
			name:    threadStateRouter,
			snap:    topology.Snapshot{State: threadStateRouter, Neighbors: []topology.Neighbor{{}}},
			label:   "Thread Router",
			variant: badgeVariantSuccess,
		},
		{
			name:    threadStateDetached,
			snap:    topology.Snapshot{State: threadStateDetached},
			label:   "Thread Detached",
			variant: badgeVariantWarning,
		},
		{
			name:    threadStateOffline,
			snap:    topology.Snapshot{State: threadStateOffline},
			label:   "Thread Offline",
			variant: badgeVariantDanger,
		},
		{
			name:    "starting",
			snap:    topology.Snapshot{},
			label:   "Thread Starting",
			variant: badgeVariantMuted,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertStatusBadge(t, ThreadBadge(tc.snap), tc.label, tc.variant)
		})
	}
}

func assertStatusBadge(t *testing.T, badge StatusBadge, wantLabel, wantVariant string) {
	t.Helper()
	if badge.Label != wantLabel {
		t.Fatalf("label = %q, want %q", badge.Label, wantLabel)
	}
	if badge.Variant != wantVariant {
		t.Fatalf("variant = %q, want %q", badge.Variant, wantVariant)
	}
	if badge.Title == "" {
		t.Fatal("expected non-empty title")
	}
	if badge.Tooltip == "" || !strings.Contains(badge.Tooltip, badge.Title) {
		t.Fatalf("tooltip = %q, want text containing title %q", badge.Tooltip, badge.Title)
	}
}
