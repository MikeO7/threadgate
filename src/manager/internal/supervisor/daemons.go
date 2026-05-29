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
