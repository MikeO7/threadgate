package radio

import (
	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

const (
	badgeVariantSuccess   = "success"
	badgeVariantWarning   = "warning"
	badgeVariantDanger    = "danger"
	badgeVariantSecondary = "secondary"
)

// ReadinessReport is the single answer to "why is my dongle red on the dashboard?"
type ReadinessReport struct {
	Host       hardware.HostAudit `json:"host"`
	RadioPath  string             `json:"radioPath"`
	ProbeError string             `json:"probeError,omitempty"`
	Version    string             `json:"version,omitempty"`
	AgentState string             `json:"agentState,omitempty"`
	AgentError string             `json:"agentError,omitempty"`
	Ready      bool               `json:"ready"`
	Summary    string             `json:"summary"`
}

// Readiness evaluates host prerequisites, USB probe, and border router agent state.
type Readiness struct {
	cfg    *config.Config
	status *runtime.Tracker
}

// NewReadiness wires a readiness checker from orchestrator config and runtime status.
func NewReadiness(cfg *config.Config, status *runtime.Tracker) *Readiness {
	return &Readiness{cfg: cfg, status: status}
}

// Check produces a structured readiness report for dashboard and health endpoints.
func (r *Readiness) Check() ReadinessReport {
	mockMode := r.cfg.Runtime.IsMock()
	report := ReadinessReport{
		Host: hardware.AuditHost(mockMode),
	}
	if r.status != nil {
		st := r.status.GetStatus()
		report.RadioPath = st.RadioPath
		report.ProbeError = st.ProbeError
		report.Version = st.ProbedVersion
		report.AgentState = st.Agent.State
		report.AgentError = st.Agent.LastError
	}
	report.Ready, report.Summary = summarize(report, mockMode)
	return report
}

func summarize(report ReadinessReport, mockMode bool) (bool, string) {
	if mockMode {
		return true, "Simulated radio — no USB Thread dongle required."
	}
	if len(report.Host.Warnings) > 0 {
		return false, "Host networking prerequisites need attention."
	}
	if report.ProbeError != "" {
		return false, "USB radio probe failed: " + report.ProbeError
	}
	if ready, summary, ok := summarizeAgentState(report); ok {
		return ready, summary
	}
	if report.Version != "" && report.RadioPath != "" {
		return true, "Radio probed successfully."
	}
	return false, "Radio readiness unknown."
}

func summarizeAgentState(report ReadinessReport) (ready bool, summary string, ok bool) {
	switch report.AgentState {
	case "waiting_for_hardware":
		if report.AgentError != "" {
			return false, report.AgentError, true
		}
		return false, "No USB Thread radio detected yet.", true
	case "restarting", "stopped":
		return false, "Border router (otbr-agent) is not running.", true
	case "running":
		if report.RadioPath != "" && report.Version != "" {
			return true, "Radio connected and border router running.", true
		}
	}
	return false, "", false
}

// BadgeFromReport derives the dashboard radio badge from a readiness report.
func BadgeFromReport(report ReadinessReport, mockMode bool) (label, variant, title string) {
	if mockMode {
		return "Simulated Radio", badgeVariantWarning, "Running with a virtual radio — no USB Thread dongle is connected."
	}
	if report.ProbeError != "" {
		return badgeForProbeError(report)
	}
	if label, variant, title, ok := badgeForAgentState(report); ok {
		return label, variant, title
	}
	if report.Version != "" && report.RadioPath != "" {
		return "Radio Connected", badgeVariantSuccess, report.Version
	}
	return "Radio Unknown", badgeVariantSecondary, report.Summary
}

func badgeForProbeError(report ReadinessReport) (label, variant, title string) {
	path := report.RadioPath
	if path == "" {
		path = "unknown device"
	}
	return "Dongle Error", badgeVariantDanger, "Could not communicate with the USB radio at " + path + ": " + report.ProbeError
}

func badgeForAgentState(report ReadinessReport) (label, variant, title string, ok bool) {
	switch report.AgentState {
	case "waiting_for_hardware":
		title := report.AgentError
		if title == "" {
			title = "No USB Thread radio detected yet. Plug in a compatible dongle."
		}
		return "Waiting for Dongle", badgeVariantWarning, title, true
	case "restarting":
		return badgeForAgentLifecycle("Border Router Restarting", badgeVariantWarning, "Border router (otbr-agent) is restarting.", report.AgentError)
	case "stopped":
		return badgeForAgentLifecycle("Border Router Stopped", badgeVariantDanger, "Border router (otbr-agent) is stopped.", report.AgentError)
	case "running":
		if report.RadioPath != "" {
			return "Radio Connected", badgeVariantSuccess, "USB Thread radio at " + report.RadioPath, true
		}
	}
	return "", "", "", false
}

func badgeForAgentLifecycle(label, variant, baseTitle, agentError string) (string, string, string, bool) {
	title := baseTitle
	if agentError != "" {
		title += " " + agentError
	}
	return label, variant, title, true
}
