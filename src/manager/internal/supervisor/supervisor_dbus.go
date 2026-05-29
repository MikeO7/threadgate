package supervisor

import (
	"log"
	"os"
	"os/exec"
	"time"
)

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
