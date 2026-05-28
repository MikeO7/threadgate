package api

import (
	"encoding/json"
	"html/template"
)

// DashboardView adds presentation fields for SSR dashboard rendering.
type DashboardView struct {
	Snapshot
	Port         int
	MockMode     bool
	CSS          template.CSS
	TopologyJS   template.JS
	TopologyJSON template.JS
}

// DashboardView builds the SSR template model for the dashboard.
func (s Snapshot) DashboardView(port int, mockMode bool) DashboardView {
	topologyJSON, _ := s.MarshalTopologyJSON()
	return DashboardView{
		Snapshot:   s,
		Port:       port,
		MockMode:   mockMode,
		CSS:        template.CSS(dashboardCSS),          //nolint:gosec // G203: embedded static stylesheet
		TopologyJS: template.JS(dashboardTopologyJS),    //nolint:gosec // G203: embedded static script
		TopologyJSON: topologyJSON,
	}
}

// MarshalTopologyJSON serializes topology data embedded in the dashboard script tag.
func (s Snapshot) MarshalTopologyJSON() (template.JS, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return template.JS(b), nil //nolint:gosec // G203: JSON from typed Snapshot struct
}
