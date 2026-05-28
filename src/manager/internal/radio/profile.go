package radio

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
)

// Profile defines the connection parameters of a serial RCP coordinator.
type Profile struct {
	DevicePath  string
	Baudrate    int
	FlowControl bool
}

// BuildSpinelURL constructs a spinel+hdlc+uart URL from a Profile.
func (p Profile) BuildSpinelURL(explicitFlowControl bool) string {
	flowParam := ""
	if p.FlowControl {
		flowParam = "&uart-flow-control=1"
	} else if explicitFlowControl {
		flowParam = "&uart-flow-control=0"
	}
	return fmt.Sprintf("spinel+hdlc+uart://%s?uart-baudrate=%d%s", p.DevicePath, p.Baudrate, flowParam)
}

// ParseSpinelURL decodes a spinel+hdlc+uart URL into a Profile.
func ParseSpinelURL(radioURL string, defaultBaud int) (Profile, bool) {
	prefix := "spinel+hdlc+uart://"
	if !strings.HasPrefix(radioURL, prefix) {
		return Profile{}, false
	}
	rawPath := strings.TrimPrefix(radioURL, prefix)

	parts := strings.Split(rawPath, "?")
	devicePath := parts[0]
	if len(parts) <= 1 {
		return Profile{
			DevicePath:  devicePath,
			Baudrate:    defaultBaud,
			FlowControl: false,
		}, true
	}

	baud := defaultBaud
	flow := false
	for _, param := range strings.Split(parts[1], "&") {
		if strings.HasPrefix(param, "uart-baudrate=") {
			val := strings.TrimPrefix(param, "uart-baudrate=")
			if b, err := strconv.Atoi(val); err == nil && b > 0 {
				baud = b
			}
		} else if strings.HasPrefix(param, "uart-flow-control=") {
			val := strings.TrimPrefix(param, "uart-flow-control=")
			flow = (val == "1" || strings.ToLower(val) == "true")
		} else if param == "uart-flow-control" {
			flow = true
		}
	}

	return Profile{
		DevicePath:  devicePath,
		Baudrate:    baud,
		FlowControl: flow,
	}, true
}

// ResolveProfile resolves the connection profile from radio Config.
func ResolveProfile(cfg Config, forceDiscover bool) (Profile, error) {
	radioURL := cfg.RadioURL
	if forceDiscover {
		radioURL = ""
	}

	if radioURL != "" {
		if profile, ok := ParseSpinelURL(radioURL, cfg.Baudrate); ok {
			return profile, nil
		}
		return Profile{
			DevicePath:  radioURL,
			Baudrate:    cfg.Baudrate,
			FlowControl: cfg.FlowControl,
		}, nil
	}

	if !cfg.AutoDiscover {
		return Profile{}, fmt.Errorf("OTBR_RADIO_URL is not set and auto-discovery is disabled")
	}

	discoveredPath, recommendedBaud, recommendedFlow, err := hardware.DiscoverRadio(cfg.MockMode)
	if err != nil {
		return Profile{}, err
	}

	baud := cfg.Baudrate
	if !cfg.ExplicitBaudrate && recommendedBaud > 0 {
		baud = recommendedBaud
	}

	flow := cfg.FlowControl
	if !cfg.ExplicitFlowControl {
		flow = recommendedFlow
	}

	return Profile{
		DevicePath:  discoveredPath,
		Baudrate:    baud,
		FlowControl: flow,
	}, nil
}
