package thread

import (
	"context"
	"strings"
	"time"
)

// HealthSummary contains comprehensive network state metrics and self-healing advice.
type HealthSummary struct {
	Timestamp       string   `json:"timestamp"`
	Healthy         bool     `json:"healthy"`
	Status          string   `json:"status"`
	State           string   `json:"state"`
	NetworkName     string   `json:"network_name,omitempty"`
	PanID           string   `json:"pan_id,omitempty"`
	NeighborCount   int      `json:"neighbor_count"`
	IPAddresses     int      `json:"ip_addresses_count"`
	Recommendations []string `json:"recommendations"`
}

// CheckHealth gathers telemetry from the Thread client and runs heuristics to evaluate network health.
func (c *Client) CheckHealth(ctx context.Context) (HealthSummary, error) {
	summary := HealthSummary{
		Timestamp:       time.Now().Format(time.RFC3339),
		Healthy:         true,
		Status:          "Excellent",
		Recommendations: []string{},
	}

	info, err := c.NodeInfo(ctx)
	if err != nil {
		return degradedHealthSummary(summary), nil
	}

	summary.State = info.State
	summary.NetworkName = info.NetworkName
	summary.PanID = info.PanID
	applyStateHealth(&summary, info.State)
	enrichHealthFromDiagnostics(ctx, &summary, c, info.State)

	summary.Recommendations = append(summary.Recommendations,
		"Ensure multicast UDP routing and mDNS are unblocked on your primary Wi-Fi Access Point so commissioning devices can discover this Border Router.",
		"For optimal stability, keep the Thread border router dongle away from heavy metal surfaces and USB 3.0 ports which cause 2.4GHz interference.")

	return summary, nil
}

func degradedHealthSummary(summary HealthSummary) HealthSummary {
	summary.Healthy = false
	summary.Status = "Degraded"
	summary.Recommendations = append(summary.Recommendations,
		"OTBR Service is unresponsive. Verify that the otbr-agent or ot-ctl socket is running and accessible.",
		"Check that the supervisor service or hardware dongle is plugged in securely.")
	return summary
}

func applyStateHealth(summary *HealthSummary, state string) {
	switch strings.ToLower(state) {
	case stateDisabled, stateOffline:
		summary.Healthy = false
		summary.Status = "Offline"
		summary.Recommendations = append(summary.Recommendations,
			"Thread interface is disabled. Execute 'ot-ctl ifconfig up' and 'ot-ctl thread start' or form a network via the dashboard.")
	case "detached":
		summary.Healthy = false
		summary.Status = "Detached"
		summary.Recommendations = append(summary.Recommendations,
			"The router is detached from the partition. Ensure there is an active Leader nearby or adjust radio/dongle positioning.")
	}
}

func enrichHealthFromDiagnostics(ctx context.Context, summary *HealthSummary, c *Client, state string) {
	diag, err := c.Diagnostics(ctx)
	if err != nil {
		return
	}
	summary.IPAddresses = len(diag.IPAddresses)
	summary.NeighborCount = countNeighborTableLines(diag.NeighborTable)
	if summary.NeighborCount == 0 && strings.ToLower(state) == mockLeader {
		summary.Status = "Idle Leader"
		summary.Recommendations = append(summary.Recommendations,
			"No child devices or neighboring routers found. To connect devices, use a Thread commissioner (e.g. mobile app) to share this network's operational dataset.")
	}
}

func countNeighborTableLines(lines []string) int {
	neighbors := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && line != mockDone && !strings.HasPrefix(line, "rloc16") && !strings.Contains(line, "---") && !strings.HasPrefix(line, "|") {
			neighbors++
		}
	}
	return neighbors
}
