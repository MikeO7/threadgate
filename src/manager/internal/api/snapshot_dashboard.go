package api

import (
	"encoding/json"
	"html/template"

	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/snapshot"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// DashboardView adds presentation fields for SSR dashboard rendering.
type DashboardView struct {
	topology.Snapshot
	Port         int
	MockMode     bool
	HassEnabled  bool
	HassStatus   string
	HassError    string
	HassURL      string
	RadioBadge   snapshot.StatusBadge
	ThreadBadge  snapshot.StatusBadge
	HassBadge    snapshot.StatusBadge
	CSS          template.CSS
	TopologyJS   template.JS
	TopologyJSON template.JS
	Status       runtime.Status
	SetupGuide   snapshot.SetupGuide
}

// NewDashboardView builds the SSR template model from a snapshot dashboard model.
func NewDashboardView(model snapshot.DashboardModel) DashboardView {
	topologyJSON, _ := MarshalTopologyJSON(model.Snapshot)
	return DashboardView{
		Snapshot:     model.Snapshot,
		Port:         model.Port,
		MockMode:     model.MockMode,
		HassEnabled:  model.HassEnabled,
		HassStatus:   model.HassStatus,
		HassError:    model.HassError,
		HassURL:      model.HassURL,
		RadioBadge:   model.RadioBadge,
		ThreadBadge:  model.ThreadBadge,
		HassBadge:    model.HassBadge,
		CSS:          template.CSS(dashboardCSS),       //nolint:gosec // G203: embedded static stylesheet
		TopologyJS:   template.JS(dashboardTopologyJS), //nolint:gosec // G203: embedded static script
		TopologyJSON: topologyJSON,
		Status:       model.Status,
		SetupGuide:   model.SetupGuide,
	}
}

// MarshalTopologyJSON serializes topology data embedded in the dashboard script tag.
func MarshalTopologyJSON(snap topology.Snapshot) (template.JS, error) {
	b, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	return template.JS(b), nil //nolint:gosec // G203: JSON from typed Snapshot struct
}
