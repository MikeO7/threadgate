// Package env is the composition root wiring config, radio, status, and Thread client.
package env

import (
	"fmt"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

// Env is the composition root: runtime mode, status, radio binding, and Thread client.
type Env struct {
	Config *config.Config
	Status *runtime.Tracker
	Radio  *radio.Binding
	Thread *thread.Client
}

// Bootstrap wires orchestration modules from loaded config.
func Bootstrap(cfg *config.Config) (*Env, error) {
	return BootstrapWithStatus(cfg, nil)
}

// BootstrapWithStatus wires orchestration modules using an optional existing status tracker.
func BootstrapWithStatus(cfg *config.Config, status *runtime.Tracker) (*Env, error) {
	if status == nil {
		status = runtime.NewTracker()
	}
	radioBinding, err := radio.NewBinding(cfg, status)
	if err != nil {
		return nil, fmt.Errorf("radio binding: %w", err)
	}

	client := thread.NewClient(thread.NewRunner(cfg.Runtime), thread.PolicyBestEffort)

	return &Env{
		Config: cfg,
		Status: status,
		Radio:  radioBinding,
		Thread: client,
	}, nil
}

// IsMock reports whether the runtime operates in mock mode.
func (e *Env) IsMock() bool {
	return e.Config.Runtime.IsMock()
}
