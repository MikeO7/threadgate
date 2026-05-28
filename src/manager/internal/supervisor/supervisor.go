// Package supervisor monitors and controls DBus, Avahi, and the C++ otbr-agent process.
//
//nolint:gocognit,nestif,revive,gocyclo,funlen // legacy daemon lifecycle management loops and complex processes
package supervisor

import (
	"context"
	"fmt"
	"log"
	"os"
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

	if err := os.MkdirAll("/run/dbus", 0750); err != nil {
		log.Printf("[Supervisor] Warning: failed to create /run/dbus: %v\n", err)
	}
	if err := os.MkdirAll("/var/run/dbus", 0750); err != nil {
		log.Printf("[Supervisor] Warning: failed to create /var/run/dbus: %v\n", err)
	}

	_ = os.Remove("/run/dbus/pid")
	_ = os.Remove("/var/run/dbus/pid")

	dbusCmd := s.launcher.CommandContext(s.ctx, "dbus-daemon", "--system", "--nofork", "--nopidfile")
	dbusCmd.Stdout = os.Stdout
	dbusCmd.Stderr = os.Stderr
	if err := dbusCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dbus-daemon: %w", err)
	}
	s.daemons.track(dbusCmd)
	log.Println("[Supervisor] dbus-daemon launched successfully.")

	// Supervise dbus-daemon in background
	go func() {
		_ = dbusCmd.Wait()
		select {
		case <-s.ctx.Done():
			return
		default:
			log.Println("[Supervisor] dbus-daemon crashed or exited. Self-healing: restarting dbus-daemon in 2 seconds...")
			time.Sleep(2 * time.Second)
			s.runDbusLoop()
		}
	}()

	time.Sleep(daemonBootDelay)

	s.startAvahi()

	go s.runAgentLoop()

	return nil
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

// Stop cleanly terminates all monitored daemons
func (s *Supervisor) Stop() {
	log.Println("[Supervisor] Initiating graceful shutdown of all processes...")
	s.setAgentStatus("stopped", "")
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	s.daemons.stopAll(daemonStopDelay)
}

func (s *Supervisor) startAvahi() {
	_ = os.Remove("/var/run/avahi-daemon/pid")
	avCmd := s.launcher.CommandContext(s.ctx, "avahi-daemon", "--no-chroot")
	avCmd.Stdout = os.Stdout
	avCmd.Stderr = os.Stderr

	if err := avCmd.Start(); err != nil {
		log.Printf("[Supervisor] Warning: avahi-daemon could not be started: %v (mDNS features may not function)\n", err)
		return
	}
	s.daemons.track(avCmd)
	log.Println("[Supervisor] avahi-daemon launched successfully.")

	go func() {
		_ = avCmd.Wait()
		select {
		case <-s.ctx.Done():
			return
		default:
			log.Println("[Supervisor] avahi-daemon crashed or exited. Self-healing: restarting avahi-daemon in 2 seconds...")
			time.Sleep(2 * time.Second)
			s.runAvahiLoop()
		}
	}()
}

func (s *Supervisor) runDbusLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			if err := os.MkdirAll("/run/dbus", 0750); err != nil {
				log.Printf("[Supervisor] Warning: failed to create /run/dbus: %v\n", err)
			}
			if err := os.MkdirAll("/var/run/dbus", 0750); err != nil {
				log.Printf("[Supervisor] Warning: failed to create /var/run/dbus: %v\n", err)
			}

			_ = os.Remove("/run/dbus/pid")
			_ = os.Remove("/var/run/dbus/pid")

			dbusCmd := s.launcher.CommandContext(s.ctx, "dbus-daemon", "--system", "--nofork", "--nopidfile")
			dbusCmd.Stdout = os.Stdout
			dbusCmd.Stderr = os.Stderr
			if err := dbusCmd.Start(); err != nil {
				log.Printf("[Supervisor] Error starting dbus-daemon: %v. Retrying in 5 seconds...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
			s.daemons.track(dbusCmd)
			log.Println("[Supervisor] dbus-daemon launched successfully.")

			_ = dbusCmd.Wait()
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Println("[Supervisor] dbus-daemon crashed or exited. Self-healing: restarting dbus-daemon in 2 seconds...")
				time.Sleep(2 * time.Second)
			}
		}
	}
}

func (s *Supervisor) runAvahiLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			_ = os.Remove("/var/run/avahi-daemon/pid")
			avCmd := s.launcher.CommandContext(s.ctx, "avahi-daemon", "--no-chroot")
			avCmd.Stdout = os.Stdout
			avCmd.Stderr = os.Stderr

			if err := avCmd.Start(); err != nil {
				log.Printf("[Supervisor] Warning: avahi-daemon could not be started: %v. Retrying in 5 seconds...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
			s.daemons.track(avCmd)
			log.Println("[Supervisor] avahi-daemon launched successfully.")

			_ = avCmd.Wait()
			select {
			case <-s.ctx.Done():
				return
			default:
				log.Println("[Supervisor] avahi-daemon crashed or exited. Self-healing: restarting avahi-daemon in 2 seconds...")
				time.Sleep(2 * time.Second)
			}
		}
	}
}

func (s *Supervisor) runAgentLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.runAgentOnce()
		}
	}
}

func (s *Supervisor) runAgentOnce() {
	cfg := s.env.Config
	targetURL := s.radio.CurrentSpinelURL()

	if cfg.AutoDiscover {
		err := s.radio.Refresh()
		if err != nil {
			if targetURL == "" {
				backoff := 2 * time.Second
				for {
					s.setAgentStatus("waiting_for_hardware", fmt.Sprintf("No radio detected: %v", err))
					log.Printf("[Supervisor] USB radio not found and no fallback URL. Waiting/polling for hardware (%v). Retrying in %v...\n", err, backoff)

					select {
					case <-s.ctx.Done():
						s.setAgentStatus("stopped", "")
						return
					case <-time.After(backoff):
					}

					err = s.radio.Refresh()
					if err == nil {
						targetURL = s.radio.CurrentSpinelURL()
						log.Printf("[Supervisor] Dynamically resolved/re-discovered radio URL: %s\n", targetURL)
						break
					}

					backoff *= 2
					if backoff > 10*time.Second {
						backoff = 10 * time.Second
					}
				}
			} else {
				log.Printf("[Supervisor] Re-discovery failed: %v. Falling back to last known URL: %s\n", err, targetURL)
			}
		} else {
			targetURL = s.radio.CurrentSpinelURL()
			log.Printf("[Supervisor] Dynamically resolved/re-discovered radio URL: %s\n", targetURL)
		}
	}

	log.Printf("[Supervisor] Launching otbr-agent with Radio: %s\n", targetURL)

	args := []string{"-I", "wpan0"}
	if cfg.BackboneIF != "" {
		args = append(args, "-B", cfg.BackboneIF)
	} else {
		args = append(args, "-B")
	}
	args = append(args, targetURL)

	s.setAgentStatus("running", "")

	agentCmd := s.launcher.CommandContext(s.ctx, "otbr-agent", args...)
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		errMsg := err.Error()
		s.setAgentStatus("restarting", errMsg)
		log.Printf("[Supervisor] Error starting otbr-agent: %v. Retrying in 5 seconds...\n", err)
		time.Sleep(5 * time.Second)
		return
	}

	s.daemons.track(agentCmd)
	log.Println("[Supervisor] otbr-agent process started successfully.")

	err := agentCmd.Wait()
	if err != nil {
		errMsg := err.Error()
		s.setAgentStatus("restarting", errMsg)
		log.Printf("[Supervisor] Warning: otbr-agent exited with error: %v\n", err)
	} else {
		s.setAgentStatus("restarting", "")
		log.Println("[Supervisor] otbr-agent exited cleanly.")
	}

	select {
	case <-s.ctx.Done():
		s.setAgentStatus("stopped", "")
		return
	default:
		log.Println("[Supervisor] Self-healing trigger: restarting otbr-agent in 3 seconds...")
		time.Sleep(3 * time.Second)
	}
}
