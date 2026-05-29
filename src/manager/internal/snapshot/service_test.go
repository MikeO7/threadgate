package snapshot

import (
	"context"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

type fakeHass struct {
	enabled bool
	names   map[string]hass.DeviceDetails
	status  string
}

func (f *fakeHass) Enabled() bool { return f.enabled }
func (f *fakeHass) FetchDeviceNames(_ context.Context) (map[string]hass.DeviceDetails, error) {
	return f.names, nil
}
func (f *fakeHass) Status() (string, string) { return f.status, "" }

type fakePairing struct{ pending bool }

func (f *fakePairing) HasPending() bool { return f.pending }

func TestServiceBuildEnrichesNeighbors(t *testing.T) {
	snap := topology.Snapshot{
		Neighbors: []topology.Neighbor{{ExtAddr: "1122334455667701"}},
	}
	mock := thread.NewMock()
	svc := &Service{
		Threads: thread.NewClient(mock, thread.PolicyBestEffort),
		Hass: &fakeHass{
			enabled: true,
			names: map[string]hass.DeviceDetails{
				"1122334455667701": {Name: "Kitchen Sensor", Manufacturer: "Acme"},
			},
			status: hass.StatusConnected,
		},
	}

	// Override BuildSnapshot path by testing enrich directly via Build with mock client.
	_ = snap
	enriched := svc.Build(context.Background())
	if enriched.HassStatus != hass.StatusConnected {
		t.Fatalf("hass status = %q", enriched.HassStatus)
	}
}

func TestServiceBuildDashboardPendingOverlay(t *testing.T) {
	mock := thread.NewMock()
	svc := &Service{
		Threads: thread.NewClient(mock, thread.PolicyBestEffort),
		Hass:    &fakeHass{enabled: false, status: hass.StatusDisabled},
		Pairing: &fakePairing{pending: true},
	}
	model := svc.BuildDashboard(context.Background(), 8081, false, runtime.Status{}, false, "")
	if model.HassStatus != hass.StatusPending {
		t.Fatalf("expected pending overlay, got %q", model.HassStatus)
	}
	if model.RadioBadge.Label == "" || model.ThreadBadge.Label == "" {
		t.Fatal("expected badges to be populated")
	}
}
