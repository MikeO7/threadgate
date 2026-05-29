package snapshot

import (
	"context"
	"log"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// Enriched holds a topology snapshot with Home Assistant integration status.
type Enriched struct {
	topology.Snapshot
	HassStatus string
	HassError  string
}

// HassReader fetches device friendly names from Home Assistant.
type HassReader interface {
	Enabled() bool
	FetchDeviceNames(ctx context.Context) (map[string]hass.DeviceDetails, error)
	Status() (string, string)
}

// PairingState exposes pending pairing requests for dashboard badge overlay.
type PairingState interface {
	HasPending() bool
}

// Service builds enriched topology snapshots for API and dashboard consumers.
type Service struct {
	Threads *thread.Client
	Hass    HassReader
	Pairing PairingState
}

// Build collects ot-ctl topology, enriches with HA names, and applies pairing overlay.
func (s *Service) Build(ctx context.Context) Enriched {
	snap, err := s.Threads.BuildSnapshot(ctx)
	if err != nil {
		log.Printf("[Snapshot] Build partial: %v\n", err)
	}
	s.enrichWithHass(ctx, &snap)

	hassStatus, hassError := "", ""
	if s.Hass != nil {
		hassStatus, hassError = s.Hass.Status()
	}
	if s.Pairing != nil && s.Pairing.HasPending() {
		hassStatus = hass.StatusPending
		hassError = ""
	}
	return Enriched{
		Snapshot:   snap,
		HassStatus: hassStatus,
		HassError:  hassError,
	}
}

func (s *Service) enrichWithHass(ctx context.Context, snap *topology.Snapshot) {
	if s.Hass == nil || !s.Hass.Enabled() {
		return
	}
	details, err := s.Hass.FetchDeviceNames(ctx)
	if err != nil {
		log.Printf("[Snapshot] Home Assistant fetch failed: %v\n", err)
		return
	}
	if len(details) == 0 {
		return
	}
	snap.DeviceNames = make(map[string]string, len(details))
	for mac, dev := range details {
		snap.DeviceNames[mac] = dev.Name
	}
	for i := range snap.Neighbors {
		normMac := hass.NormalizeMac(snap.Neighbors[i].ExtAddr)
		if dev, ok := details[normMac]; ok {
			snap.Neighbors[i].FriendlyName = dev.Name
			snap.Neighbors[i].Manufacturer = dev.Manufacturer
			snap.Neighbors[i].Model = dev.Model
			snap.Neighbors[i].SwVersion = dev.SwVersion
			snap.Neighbors[i].Battery = dev.Battery
			snap.Neighbors[i].Availability = dev.Availability
			snap.Neighbors[i].HassDeviceID = dev.DeviceID
		}
	}
}
