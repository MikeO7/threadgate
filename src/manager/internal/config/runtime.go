package config

// RuntimeMode distinguishes hardware production runs from simulated mock runs.
type RuntimeMode int

const (
	// RuntimeModeHardware runs against real otbr-agent and USB radio hardware.
	RuntimeModeHardware RuntimeMode = iota
	// RuntimeModeMock simulates ot-ctl, supervisor daemons, and radio discovery.
	RuntimeModeMock
)

// RuntimeModeFromMock returns the runtime mode for a mock-mode flag.
func RuntimeModeFromMock(mockMode bool) RuntimeMode {
	if mockMode {
		return RuntimeModeMock
	}
	return RuntimeModeHardware
}

// IsMock reports whether the runtime operates in mock mode.
func (m RuntimeMode) IsMock() bool {
	return m == RuntimeModeMock
}
