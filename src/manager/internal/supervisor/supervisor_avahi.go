package supervisor

import (
	"log"
	"os"
	"os/exec"
	"time"
)

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
