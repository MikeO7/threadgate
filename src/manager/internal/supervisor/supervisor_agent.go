package supervisor

import (
	"fmt"
	"log"
	"os"
	"time"
)

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
		time.Sleep(5 * time.Second)
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
	default:
		log.Println("[Supervisor] Self-healing trigger: restarting otbr-agent in 3 seconds...")
		time.Sleep(3 * time.Second)
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
