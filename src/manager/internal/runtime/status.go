// Package runtime tracks orchestrator health and host audit state.
package runtime

import (
	"sync"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
)

// AgentStatus reports otbr-agent lifecycle from the supervisor module.
type AgentStatus struct {
	State     string `json:"state"` // running, stopped, restarting, mock
	LastError string `json:"lastError,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Status is orchestration-wide health: host audit, radio probe, and agent lifecycle.
type Status struct {
	HostAudit      hardware.HostAudit `json:"hostAudit"`
	ProbedVersion  string             `json:"probedVersion"`
	ProbeError     string             `json:"probeError,omitempty"`
	RadioPath      string             `json:"radioPath,omitempty"`
	Agent          AgentStatus        `json:"agent"`
	DetectedDevice string             `json:"detectedDevice,omitempty"`
}

// Reporter provides read-only access to orchestration status (/api/health).
type Reporter interface {
	GetStatus() Status
}

// Tracker collects status updates from app, radio binding, and supervisor.
type Tracker struct {
	mu     sync.RWMutex
	status Status
}

// NewTracker creates an empty status tracker.
func NewTracker() *Tracker {
	return &Tracker{}
}

// GetStatus implements Reporter.
func (t *Tracker) GetStatus() Status {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// SetHostAudit replaces host audit fields (typically once at boot).
func (t *Tracker) SetHostAudit(audit hardware.HostAudit) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.HostAudit = audit
}

// UpdateRadioHealth updates radio path and probe results.
func (t *Tracker) UpdateRadioHealth(path, version, errStr, detected string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.RadioPath = path
	t.status.ProbedVersion = version
	t.status.ProbeError = errStr
	t.status.DetectedDevice = detected
}

// UpdateAgent records supervisor agent lifecycle transitions.
func (t *Tracker) UpdateAgent(state, lastErr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Agent = AgentStatus{
		State:     state,
		LastError: lastErr,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}
