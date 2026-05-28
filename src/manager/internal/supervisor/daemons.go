// Package supervisor manages otbr-agent and supporting system daemons.
package supervisor

import (
	"os/exec"
	"sync"
	"syscall"
	"time"
)

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
	r.mu.Lock()
	procs := append([]*managedProcess(nil), r.procs...)
	r.procs = nil
	r.mu.Unlock()

	deadline := time.Now().Add(grace)
	for _, proc := range procs {
		if proc == nil || proc.cmd == nil || proc.cmd.Process == nil {
			continue
		}
		_ = proc.cmd.Process.Signal(syscall.SIGTERM)
	}

	for _, proc := range procs {
		if proc == nil {
			continue
		}
		select {
		case <-proc.done:
		case <-time.After(time.Until(deadline)):
			if proc.cmd != nil && proc.cmd.Process != nil {
				_ = proc.cmd.Process.Kill()
			}
		}
	}
}

