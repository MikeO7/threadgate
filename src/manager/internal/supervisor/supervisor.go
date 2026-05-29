// Package supervisor monitors and controls DBus, Avahi, and the C++ otbr-agent process.
package supervisor

import (
	"context"
	"fmt"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
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
		log.Printf("[Supervisor] Warning: failed to start dbus-daemon: %v. Proceeding without host D-Bus.\n", err)
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

var (
	daemonBootDelay = 1 * time.Second
	daemonStopDelay = 2 * time.Second
)

type managedProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
}

type processRegistry struct {
	mu    sync.Mutex
	procs []*managedProcess
}

func (r *processRegistry) track(cmd *exec.Cmd) chan struct{} {
	if cmd == nil {
		return nil
	}
	done := make(chan struct{})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.procs = append(r.procs, &managedProcess{cmd: cmd, done: done})
	return done
}

func (r *processRegistry) stopAll(grace time.Duration) {
	procs := r.takeProcesses()
	sendTermSignals(procs)
	waitForTermination(procs, time.Now().Add(grace))
}

func (r *processRegistry) takeProcesses() []*managedProcess {
	r.mu.Lock()
	defer r.mu.Unlock()
	procs := append([]*managedProcess(nil), r.procs...)
	r.procs = nil
	return procs
}

func sendTermSignals(procs []*managedProcess) {
	for _, proc := range procs {
		if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
			continue
		}
		_ = proc.cmd.Process.Signal(syscall.SIGTERM)
	}
}

func waitForTermination(procs []*managedProcess, deadline time.Time) {
	for _, proc := range procs {
		if proc == nil {
			continue
		}
		select {
		case <-proc.done:
		case <-time.After(time.Until(deadline)):
			killProcess(proc)
		}
	}
}

func killProcess(proc *managedProcess) {
	if proc.cmd != nil && proc.cmd.Process != nil {
		_ = proc.cmd.Process.Kill()
	}
}

// ProcessLauncher starts subprocesses (exec adapter in prod, fakes in tests).
type ProcessLauncher interface {
	CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd
}

// ExecLauncher runs real subprocesses via os/exec.
type ExecLauncher struct{}

func (ExecLauncher) CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, arg...) //nolint:gosec // ProcessLauncher intentionally runs configured daemon binaries.
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
	targetURL := s.resolveAgentRadioURL()
	if targetURL == "" {
		return
	}

	log.Printf("[Supervisor] Launching otbr-agent with Radio: %s\n", targetURL)
	s.setAgentStatus("running", "")

	agentCmd := s.launcher.CommandContext(s.ctx, "otbr-agent", s.buildAgentArgs(targetURL)...)
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		s.setAgentStatus("restarting", err.Error())
		log.Printf("[Supervisor] Error starting otbr-agent: %v. Retrying in 5 seconds...\n", err)
		select {
		case <-s.ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return
	}

	done := s.daemons.track(agentCmd)
	log.Println("[Supervisor] otbr-agent process started successfully.")

	err := agentCmd.Wait()
	close(done)
	s.recordAgentExit(err)

	select {
	case <-s.ctx.Done():
		s.setAgentStatus("stopped", "")
		return
	case <-time.After(3 * time.Second):
		log.Println("[Supervisor] Self-healing trigger: restarting otbr-agent...")
	}
}

func (s *Supervisor) resolveAgentRadioURL() string {
	targetURL := s.radio.CurrentSpinelURL()
	if !s.env.Config.AutoDiscover {
		return targetURL
	}

	err := s.radio.Refresh()
	if err == nil {
		targetURL = s.radio.CurrentSpinelURL()
		log.Printf("[Supervisor] Dynamically resolved/re-discovered radio URL: %s\n", targetURL)
		return targetURL
	}

	if targetURL != "" {
		log.Printf("[Supervisor] Re-discovery failed: %v. Falling back to last known URL: %s\n", err, targetURL)
		return targetURL
	}

	return s.waitForRadioHardware(err)
}

func (s *Supervisor) waitForRadioHardware(initialErr error) string {
	backoff := 2 * time.Second
	err := initialErr
	for {
		s.setAgentStatus("waiting_for_hardware", fmt.Sprintf("No radio detected: %v", err))
		log.Printf("[Supervisor] USB radio not found and no fallback URL. Waiting/polling for hardware (%v). Retrying in %v...\n", err, backoff)

		select {
		case <-s.ctx.Done():
			s.setAgentStatus("stopped", "")
			return ""
		case <-time.After(backoff):
		}

		err = s.radio.Refresh()
		if err == nil {
			targetURL := s.radio.CurrentSpinelURL()
			log.Printf("[Supervisor] Dynamically resolved/re-discovered radio URL: %s\n", targetURL)
			return targetURL
		}

		backoff *= 2
		if backoff > 10*time.Second {
			backoff = 10 * time.Second
		}
	}
}

func (s *Supervisor) buildAgentArgs(targetURL string) []string {
	args := []string{"-I", "wpan0"}
	if s.env.Config.BackboneIF != "" {
		args = append(args, "-B", s.env.Config.BackboneIF)
	} else {
		args = append(args, "-B")
	}
	return append(args, targetURL)
}

func (s *Supervisor) recordAgentExit(err error) {
	if err != nil {
		s.setAgentStatus("restarting", err.Error())
		log.Printf("[Supervisor] Warning: otbr-agent exited with error: %v\n", err)
		return
	}
	s.setAgentStatus("restarting", "")
	log.Println("[Supervisor] otbr-agent exited cleanly.")
}

func (s *Supervisor) startAvahi() {
	_ = os.Remove("/var/run/avahi-daemon/pid")
	cmd, done, err := s.launchAvahi()
	if err != nil {
		log.Printf("[Supervisor] Warning: avahi-daemon could not be started: %v (mDNS features may not function)\n", err)
		return
	}
	go s.superviseAvahi(cmd, done)
}

func (s *Supervisor) launchAvahi() (*exec.Cmd, chan struct{}, error) {
	cmd := s.launcher.CommandContext(s.ctx, "avahi-daemon", "--no-chroot")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := s.daemons.track(cmd)
	log.Println("[Supervisor] avahi-daemon launched successfully.")
	return cmd, done, nil
}

func (s *Supervisor) superviseAvahi(cmd *exec.Cmd, done chan struct{}) {
	_ = cmd.Wait()
	close(done)
	select {
	case <-s.ctx.Done():
		return
	default:
		log.Println("[Supervisor] avahi-daemon crashed or exited. Self-healing: restarting avahi-daemon in 2 seconds...")
		time.Sleep(2 * time.Second)
		s.runAvahiLoop()
	}
}

func (s *Supervisor) runAvahiLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			_ = os.Remove("/var/run/avahi-daemon/pid")
			cmd, done, err := s.launchAvahi()
			if err != nil {
				log.Printf("[Supervisor] Warning: avahi-daemon could not be started: %v. Retrying in 5 seconds...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
			_ = cmd.Wait()
			close(done)
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

func (s *Supervisor) startDbus() error {
	s.prepareDbusRuntime()
	cmd, done, err := s.launchDbus()
	if err != nil {
		return err
	}
	go s.superviseDbus(cmd, done)
	return nil
}

func (s *Supervisor) prepareDbusRuntime() {
	if err := os.MkdirAll("/run/dbus", 0750); err != nil {
		log.Printf("[Supervisor] Warning: failed to create /run/dbus: %v\n", err)
	}
	if err := os.MkdirAll("/var/run/dbus", 0750); err != nil {
		log.Printf("[Supervisor] Warning: failed to create /var/run/dbus: %v\n", err)
	}
	_ = os.Remove("/run/dbus/pid")
	_ = os.Remove("/var/run/dbus/pid")
}

func (s *Supervisor) launchDbus() (*exec.Cmd, chan struct{}, error) {
	cmd := s.launcher.CommandContext(s.ctx, "dbus-daemon", "--system", "--nofork", "--nopidfile")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := s.daemons.track(cmd)
	log.Println("[Supervisor] dbus-daemon launched successfully.")
	return cmd, done, nil
}

func (s *Supervisor) superviseDbus(cmd *exec.Cmd, done chan struct{}) {
	_ = cmd.Wait()
	close(done)
	select {
	case <-s.ctx.Done():
		return
	default:
		log.Println("[Supervisor] dbus-daemon crashed or exited. Self-healing: restarting dbus-daemon in 2 seconds...")
		time.Sleep(2 * time.Second)
		s.runDbusLoop()
	}
}

func (s *Supervisor) runDbusLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			s.prepareDbusRuntime()
			cmd, done, err := s.launchDbus()
			if err != nil {
				log.Printf("[Supervisor] Error starting dbus-daemon: %v. Retrying in 5 seconds...\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
			_ = cmd.Wait()
			close(done)
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
