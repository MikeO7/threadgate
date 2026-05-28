// Package supervisor implements daemon monitoring and lifecycle management for background services.
package supervisor

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

// Supervisor monitors and controls DBus, Avahi, and the C++ otbr-agent process.
type Supervisor struct {
	radioURL   string
	stateDir   string
	logLevel   string
	backboneIF string
	mockMode   bool
	cancelFunc context.CancelFunc
	ctx        context.Context
}

// New creates a new supervisor instance.
func New(radioURL, stateDir, logLevel string, mode config.RuntimeMode, backboneIF string) *Supervisor {
	return &Supervisor{
		radioURL:   radioURL,
		stateDir:   stateDir,
		logLevel:   logLevel,
		backboneIF: backboneIF,
		mockMode:   mode.IsMock(),
	}
}

// Start spawns the supervisor lifecycle
func (s *Supervisor) Start(ctx context.Context) error {
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	log.Println("[Supervisor] Bootstrapping system daemons...")

	if s.mockMode {
		log.Println("[Supervisor] Mock mode active: starting simulated system daemons...")
		go s.runMockAgentLoop()
		return nil
	}

	// 1. Ensure DBus system directories exist inside container
	if err := os.MkdirAll("/run/dbus", 0750); err != nil {
		log.Printf("[Supervisor] Warning: failed to create /run/dbus: %v\n", err)
	}
	if err := os.MkdirAll("/var/run/dbus", 0750); err != nil {
		log.Printf("[Supervisor] Warning: failed to create /var/run/dbus: %v\n", err)
	}

	// Clean up any stale dbus pids
	_ = os.Remove("/run/dbus/pid")
	_ = os.Remove("/var/run/dbus/pid")

	// 2. Spawn dbus-daemon
	dbusCmd := exec.CommandContext(s.ctx, "dbus-daemon", "--system", "--nofork", "--nopidfile")
	dbusCmd.Stdout = os.Stdout
	dbusCmd.Stderr = os.Stderr
	if err := dbusCmd.Start(); err != nil {
		return fmt.Errorf("failed to start dbus-daemon: %w", err)
	}
	log.Println("[Supervisor] dbus-daemon launched successfully.")

	// Give DBus a moment to initialize the socket
	time.Sleep(1 * time.Second)

	// 3. Spawn avahi-daemon (optional, handle missing gracefully)
	s.startAvahi()

	// 4. Spawn otbr-agent in a supervision loop
	go s.runAgentLoop()

	return nil
}

func (s *Supervisor) runMockAgentLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			log.Println("[Supervisor] Launching simulated otbr-agent...")
			select {
			case <-s.ctx.Done():
				log.Println("[Supervisor] Simulated otbr-agent exited cleanly.")
				return
			case <-time.After(10 * time.Minute):
			}
		}
	}
}

// Stop cleanly terminates all monitored daemons
func (s *Supervisor) Stop() {
	log.Println("[Supervisor] Initiating graceful shutdown of all processes...")
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	// Give subprocesses time to clean up
	time.Sleep(2 * time.Second)
}

func (s *Supervisor) startAvahi() {
	// Clean up any stale avahi socket
	_ = os.Remove("/var/run/avahi-daemon/pid")
	avCmd := exec.CommandContext(s.ctx, "avahi-daemon", "--no-chroot")
	avCmd.Stdout = os.Stdout
	avCmd.Stderr = os.Stderr

	if err := avCmd.Start(); err != nil {
		log.Printf("[Supervisor] Warning: avahi-daemon could not be started: %v (mDNS features may not function)\n", err)
		return
	}
	log.Println("[Supervisor] avahi-daemon launched successfully.")
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

// runAgentOnce executes a single run of the otbr-agent process and manages its lifecycle.
func (s *Supervisor) runAgentOnce() {
	log.Printf("[Supervisor] Launching otbr-agent with Radio: %s\n", s.radioURL)

	// Build arguments for the C++ otbr-agent
	// -I wpan0: Bind to interface wpan0
	// -B: Backbone interface for border routing
	args := []string{"-I", "wpan0"}
	if s.backboneIF != "" {
		args = append(args, "-B", s.backboneIF)
	} else {
		args = append(args, "-B")
	}

	// Append target radio URL
	args = append(args, s.radioURL)

	agentCmd := exec.CommandContext(s.ctx, "otbr-agent", args...)
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	// exec.CommandContext automatically handles signaling child processes on context cancellation.

	if err := agentCmd.Start(); err != nil {
		log.Printf("[Supervisor] Error starting otbr-agent: %v. Retrying in 5 seconds...\n", err)
		time.Sleep(5 * time.Second)
		return
	}

	log.Println("[Supervisor] otbr-agent process started successfully.")

	// Wait for the agent to exit
	err := agentCmd.Wait()
	if err != nil {
		log.Printf("[Supervisor] Warning: otbr-agent exited with error: %v\n", err)
	} else {
		log.Println("[Supervisor] otbr-agent exited cleanly.")
	}

	// Self-healing: if we are not shutting down, retry launching the process
	select {
	case <-s.ctx.Done():
		return
	default:
		log.Println("[Supervisor] Self-healing trigger: restarting otbr-agent in 3 seconds...")
		time.Sleep(3 * time.Second)
	}
}
