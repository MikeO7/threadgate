package api

import (
	"encoding/json"
	"html/template"

	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// DashboardView adds presentation fields for SSR dashboard rendering.
type DashboardView struct {
	topology.Snapshot
	Port         int
	MockMode     bool
	HassEnabled  bool
	CSS          template.CSS
	TopologyJS   template.JS
	TopologyJSON template.JS
	Status       runtime.Status
}

// DashboardView builds the SSR template model for the dashboard.
func NewDashboardView(snap topology.Snapshot, port int, mockMode bool, status runtime.Status, hassEnabled bool) DashboardView {
	topologyJSON, _ := MarshalTopologyJSON(snap)
	return DashboardView{
		Snapshot:     snap,
		Port:         port,
		MockMode:     mockMode,
		HassEnabled:  hassEnabled,
		CSS:          template.CSS(dashboardCSS),       //nolint:gosec // G203: embedded static stylesheet
		TopologyJS:   template.JS(dashboardTopologyJS), //nolint:gosec // G203: embedded static script
		TopologyJSON: topologyJSON,
		Status:       status,
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
