package radio

import "github.com/MikeO7/threadgate/src/manager/internal/config"

// Config holds only radio-relevant orchestrator settings.
type Config struct {
	RadioURL            string
	AutoDiscover        bool
	Baudrate            int
	ExplicitBaudrate    bool
	FlowControl         bool
	ExplicitFlowControl bool
	MockMode            bool
}

// ConfigFrom extracts radio settings from the full orchestrator config.
func ConfigFrom(cfg *config.Config) Config {
	return Config{
		RadioURL:            cfg.RadioURL,
		AutoDiscover:        cfg.AutoDiscover,
		Baudrate:            cfg.Baudrate,
		ExplicitBaudrate:    cfg.ExplicitBaudrate,
		FlowControl:         cfg.FlowControl,
		ExplicitFlowControl: cfg.ExplicitFlowControl,
		MockMode:            cfg.Runtime.IsMock(),
	}
}
