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

type processRegistry struct {
	mu    sync.Mutex
	procs []*exec.Cmd
}

func (r *processRegistry) track(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.procs = append(r.procs, cmd)
}

func (r *processRegistry) stopAll(grace time.Duration) {
	r.mu.Lock()
	procs := append([]*exec.Cmd(nil), r.procs...)
	r.procs = nil
	r.mu.Unlock()

	deadline := time.Now().Add(grace)
	for _, cmd := range procs {
		if cmd == nil || cmd.Process == nil {
			continue
		}
		_ = cmd.Process.Signal(syscall.SIGTERM)
	}

	for _, cmd := range procs {
		if cmd == nil {
			continue
		}
		waitDone := make(chan struct{})
		go func(c *exec.Cmd) {
			_ = c.Wait()
			close(waitDone)
		}(cmd)
		select {
		case <-waitDone:
		case <-time.After(time.Until(deadline)):
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	}
}
