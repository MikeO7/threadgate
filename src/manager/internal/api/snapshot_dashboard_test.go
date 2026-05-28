package api

import (
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/api/topology"
)

func TestNewDashboardView(t *testing.T) {
	snap := topology.BuildFromTables("0xc000", "", "", "")
	view := NewDashboardView(snap, 9090, true)
	if view.Port != 9090 || !view.MockMode {
		t.Fatalf("unexpected dashboard view: port=%d mock=%t", view.Port, view.MockMode)
	}
	if len(view.TopologyJSON) == 0 {
		t.Fatal("expected embedded topology JSON")
	}
}
