package radio

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

// radioConfig holds only radio-relevant orchestrator settings.
type radioConfig struct {
	RadioURL            string
	AutoDiscover        bool
	Baudrate            int
	ExplicitBaudrate    bool
	FlowControl         bool
	ExplicitFlowControl bool
	MockMode            bool
}

// configFrom extracts radio settings from the full orchestrator config.
func configFrom(cfg *config.Config) radioConfig {
	return radioConfig{
		RadioURL:            cfg.RadioURL,
		AutoDiscover:        cfg.AutoDiscover,
		Baudrate:            cfg.Baudrate,
		ExplicitBaudrate:    cfg.ExplicitBaudrate,
		FlowControl:         cfg.FlowControl,
		ExplicitFlowControl: cfg.ExplicitFlowControl,
		MockMode:            cfg.Runtime.IsMock(),
	}
}

// profile defines the connection parameters of a serial RCP coordinator.
type profile struct {
	DevicePath  string
	Baudrate    int
	FlowControl bool
}

// buildSpinelURL constructs a spinel+hdlc+uart URL from a profile.
func (p profile) buildSpinelURL(explicitFlowControl bool) string {
	flowParam := ""
	if p.FlowControl {
		flowParam = "&uart-flow-control=1"
	} else if explicitFlowControl {
		flowParam = "&uart-flow-control=0"
	}
	return fmt.Sprintf("spinel+hdlc+uart://%s?uart-baudrate=%d%s", p.DevicePath, p.Baudrate, flowParam)
}

// parseSpinelURL decodes a spinel+hdlc+uart URL into a profile.
func parseSpinelURL(radioURL string, defaultBaud int) (profile, bool) {
	prefix := "spinel+hdlc+uart://"
	if !strings.HasPrefix(radioURL, prefix) {
		return profile{}, false
	}
	rawPath := strings.TrimPrefix(radioURL, prefix)

	parts := strings.Split(rawPath, "?")
	devicePath := parts[0]
	if len(parts) <= 1 {
		return profile{
			DevicePath:  devicePath,
			Baudrate:    defaultBaud,
			FlowControl: false,
		}, true
	}

	baud := defaultBaud
	flow := false
	for _, param := range strings.Split(parts[1], "&") {
		switch {
		case strings.HasPrefix(param, "uart-baudrate="):
			val := strings.TrimPrefix(param, "uart-baudrate=")
			if b, err := strconv.Atoi(val); err == nil && b > 0 {
				baud = b
			}
		case strings.HasPrefix(param, "uart-flow-control="):
			val := strings.TrimPrefix(param, "uart-flow-control=")
			flow = (val == "1" || strings.ToLower(val) == "true")
		case param == "uart-flow-control":
			flow = true
		}
	}

	return profile{
		DevicePath:  devicePath,
		Baudrate:    baud,
		FlowControl: flow,
	}, true
}

// resolveProfile resolves the connection profile from radio Config.
func resolveProfile(cfg radioConfig, forceDiscover bool) (profile, error) {
	radioURL := cfg.RadioURL
	if forceDiscover {
		radioURL = ""
	}

	if radioURL != "" {
		if p, ok := parseSpinelURL(radioURL, cfg.Baudrate); ok {
			return p, nil
		}
		return profile{
			DevicePath:  radioURL,
			Baudrate:    cfg.Baudrate,
			FlowControl: cfg.FlowControl,
		}, nil
	}

	if !cfg.AutoDiscover {
		return profile{}, fmt.Errorf("OTBR_RADIO_URL is not set and auto-discovery is disabled")
	}

	discoveredPath, recommendedBaud, recommendedFlow, err := hardware.DiscoverRadio(cfg.MockMode)
	if err != nil {
		return profile{}, err
	}

	baud := cfg.Baudrate
	if !cfg.ExplicitBaudrate && recommendedBaud > 0 {
		baud = recommendedBaud
	}

	flow := cfg.FlowControl
	if !cfg.ExplicitFlowControl {
		flow = recommendedFlow
	}

	return profile{
		DevicePath:  discoveredPath,
		Baudrate:    baud,
		FlowControl: flow,
	}, nil
}

// Binding owns radio resolution, probing, and runtime status updates.
type Binding struct {
	cfg      radioConfig
	status   *runtime.Tracker

	mu         sync.RWMutex
	spinelURL  string
	devicePath string
}

// NewBinding resolves the initial radio URL, probes serial hardware, and updates status.
func NewBinding(cfg *config.Config, status *runtime.Tracker) (*Binding, error) {
	b := &Binding{
		cfg:      configFrom(cfg),
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
	p, err := resolveProfile(b.cfg, forceDiscover)
	if err != nil {
		return fmt.Errorf("%v (please set OTBR_RADIO_URL explicitly)", err)
	}

	url := p.buildSpinelURL(b.cfg.ExplicitFlowControl)
	b.mu.Lock()
	b.spinelURL = url
	b.devicePath = p.DevicePath
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

func probe(cfg radioConfig, radioURL string) (probedVersion, devicePath string, probeErr error) {
	p, isSerial := parseSpinelURL(radioURL, cfg.Baudrate)
	if !isSerial {
		log.Println("[Radio] Network-based or non-serial RCP detected. Skipping serial pre-flight hardware probe.")
		return "", "", nil
	}

	devicePath = p.DevicePath
	baudrate := p.Baudrate

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
