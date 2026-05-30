package snapshot

import (
	"context"

	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

// DashboardModel is the enriched network view for dashboard and topology consumers.
type DashboardModel struct {
	Enriched
	Port        int
	MockMode    bool
	HassEnabled bool
	HassURL     string
	Status      runtime.Status
	SetupGuide  SetupGuide
	RadioBadge  StatusBadge
	ThreadBadge StatusBadge
	HassBadge   StatusBadge
}

// BuildDashboard collects topology, enriches with HA names, and derives header badges.
func (s *Service) BuildDashboard(
	ctx context.Context,
	port int,
	mockMode bool,
	mockSetupChecklist bool,
	status runtime.Status,
	hassEnabled bool,
	hassURL string,
) DashboardModel {
	enriched := s.Build(ctx)
	hostAudit := hardware.AuditHost(mockMode)
	return DashboardModel{
		Enriched:    enriched,
		Port:        port,
		MockMode:    mockMode,
		HassEnabled: hassEnabled,
		HassURL:     hassURL,
		Status:      status,
		SetupGuide:  BuildSetupGuide(mockMode, mockSetupChecklist, hostAudit, status),
		RadioBadge:  RadioBadge(status, mockMode),
		ThreadBadge: ThreadBadge(enriched.Snapshot),
		HassBadge:   HassBadge(enriched.HassStatus, enriched.HassError),
	}
}
