package radio

import (
	"fmt"
	"log"
	"sync"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

// Binding owns radio resolution, probing, and runtime status updates.
type Binding struct {
	cfg      Config
	resolver *Resolver
	status   *runtime.Tracker

	mu         sync.RWMutex
	spinelURL  string
	devicePath string
}

// NewBinding resolves the initial radio URL, probes serial hardware, and updates status.
func NewBinding(cfg *config.Config, status *runtime.Tracker) (*Binding, error) {
	b := &Binding{
		cfg:      ConfigFrom(cfg),
		resolver: NewResolver(cfg),
		status:   status,
	}
	if err := b.resolve(false); err != nil {
		return nil, err
	}
	b.probeAndUpdateStatus()
	return b, nil
}

// CurrentSpinelURL returns the active spinel URL for otbr-agent.
func (b *Binding) CurrentSpinelURL() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.spinelURL
}

// Refresh re-resolves the radio when auto-discovery is enabled and re-probes serial hardware.
func (b *Binding) Refresh() error {
	if err := b.resolve(b.cfg.AutoDiscover); err != nil {
		return err
	}
	b.probeAndUpdateStatus()
	return nil
}

func (b *Binding) resolve(forceDiscover bool) error {
	profile, err := b.resolver.Resolve(forceDiscover)
	if err != nil {
		return fmt.Errorf("%v (please set OTBR_RADIO_URL explicitly)", err)
	}

	url := profile.BuildSpinelURL(b.cfg.ExplicitFlowControl)
	b.mu.Lock()
	b.spinelURL = url
	b.devicePath = profile.DevicePath
	b.mu.Unlock()
	return nil
}

func (b *Binding) probeAndUpdateStatus() {
	b.mu.RLock()
	url := b.spinelURL
	b.mu.RUnlock()

	version, path, probeErr := probe(b.cfg, url)
	errStr := ""
	if probeErr != nil {
		errStr = probeErr.Error()
	}
	if path == "" {
		b.mu.RLock()
		path = b.devicePath
		b.mu.RUnlock()
	}

	if b.status != nil {
		b.status.UpdateRadioHealth(path, version, errStr)
	}
}

func probe(cfg Config, radioURL string) (probedVersion, devicePath string, probeErr error) {
	profile, isSerial := ParseSpinelURL(radioURL, cfg.Baudrate)
	if !isSerial {
		log.Println("[Radio] Network-based or non-serial RCP detected. Skipping serial pre-flight hardware probe.")
		return "", "", nil
	}

	devicePath = profile.DevicePath
	baudrate := profile.Baudrate

	if cfg.MockMode {
		probedVersion = "ThreadGateMock/1.0.0; SIMULATION; May 28 2026"
		log.Printf("[Radio] Mock mode active: skipping hardware probe. Probed version set to simulated: %s\n", probedVersion)
		return probedVersion, devicePath, nil
	}

	log.Printf("[Radio] Probing physical radio %s at %d baud...\n", devicePath, baudrate)
	probedVersion, probeErr = hardware.ProbeDevice(devicePath, baudrate)
	if probeErr != nil {
		log.Printf("[Radio] Pre-flight radio probe failed: %v\n", probeErr)
	} else {
		log.Printf("[Radio] Pre-flight radio probe succeeded. RCP Version: %s\n", probedVersion)
	}

	return probedVersion, devicePath, probeErr
}
