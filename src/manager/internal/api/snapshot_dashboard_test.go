package api

import (
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/snapshot"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

func TestNewDashboardView(t *testing.T) {
	snap := topology.BuildFromTables("0xc000", "", "", "")
	model := snapshot.DashboardModel{
		Enriched: snapshot.Enriched{
			Snapshot:   snap,
			HassStatus: hass.StatusConnected,
		},
		Port:        9090,
		MockMode:    true,
		HassEnabled: true,
		HassURL:     "http://localhost:8123",
		Status:      runtime.Status{},
		RadioBadge:  snapshot.RadioBadge(runtime.Status{}, true),
		ThreadBadge: snapshot.ThreadBadge(snap),
		HassBadge:   snapshot.HassBadge(hass.StatusConnected, ""),
	}
	view := NewDashboardView(model)
	if view.Port != 9090 || !view.MockMode || !view.HassEnabled {
		t.Fatalf("unexpected dashboard view: port=%d mock=%t hass=%t", view.Port, view.MockMode, view.HassEnabled)
	}
	if view.RadioBadge.Label != "Simulated Radio" {
		t.Fatalf("expected simulated radio badge, got %q", view.RadioBadge.Label)
	}
	if view.HassBadge.Label != "HASS Connected" {
		t.Fatalf("expected hass connected badge, got %q", view.HassBadge.Label)
	}
	if len(view.TopologyJSON) == 0 {
		t.Fatal("expected embedded topology JSON")
	}
}
