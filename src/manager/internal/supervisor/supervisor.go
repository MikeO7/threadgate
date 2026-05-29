// Package supervisor monitors and controls DBus, Avahi, and the C++ otbr-agent process.
package supervisor

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
)

// mockAgentSleep controls how long the simulated otbr-agent loop waits (overridable in tests).
var mockAgentSleep = 10 * time.Minute

// Supervisor monitors and controls DBus, Avahi, and the C++ otbr-agent process.
type Supervisor struct {
	env      *env.Env
	radio    *radio.Binding
	status   *runtime.Tracker
	launcher ProcessLauncher
	daemons  processRegistry

	cancelFunc context.CancelFunc
	ctx        context.Context
}

// New creates a new supervisor instance.
func New(e *env.Env, launcher ProcessLauncher) *Supervisor {
	if launcher == nil {
		launcher = ExecLauncher{}
	}
	return &Supervisor{
		env:      e,
		radio:    e.Radio,
		status:   e.Status,
		launcher: launcher,
	}
}

// Start spawns the supervisor lifecycle
func (s *Supervisor) Start(ctx context.Context) error {
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	log.Println("[Supervisor] Bootstrapping system daemons...")

	if s.env.IsMock() {
		log.Println("[Supervisor] Mock mode active: starting simulated system daemons...")
		s.setAgentStatus("mock", "")
		go s.runMockAgentLoop()
		return nil
	}

	if err := s.startDbus(); err != nil {
		return fmt.Errorf("failed to start dbus-daemon: %w", err)
	}

	time.Sleep(daemonBootDelay)
	s.startAvahi()
	go s.runAgentLoop()

	return nil
}

// Stop cleanly terminates all monitored daemons
func (s *Supervisor) Stop() {
	log.Println("[Supervisor] Initiating graceful shutdown of all processes...")
	s.setAgentStatus("stopped", "")
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	s.daemons.stopAll(daemonStopDelay)
}

func (s *Supervisor) setAgentStatus(state, lastErr string) {
	if s.status != nil {
		s.status.UpdateAgent(state, lastErr)
	}
}

func (s *Supervisor) runMockAgentLoop() {
	for {
		select {
		case <-s.ctx.Done():
			s.setAgentStatus("stopped", "")
			return
		default:
			s.setAgentStatus("mock", "")
			log.Println("[Supervisor] Launching simulated otbr-agent...")
			select {
			case <-s.ctx.Done():
				log.Println("[Supervisor] Simulated otbr-agent exited cleanly.")
				s.setAgentStatus("stopped", "")
				return
			case <-time.After(mockAgentSleep):
			}
		}
	}
}
