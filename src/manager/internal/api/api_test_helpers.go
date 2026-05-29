package api

import (
	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

func testEnv(runner otctl.Runner, mockMode bool) *env.Env {
	return testEnvWithStatus(runner, mockMode, nil)
}

func testEnvWithStatus(runner otctl.Runner, mockMode bool, status *runtime.Tracker) *env.Env {
	return &env.Env{
		Config: &config.Config{Runtime: config.RuntimeModeFromMock(mockMode)},
		Status: status,
		Thread: thread.NewClient(runner, thread.PolicyBestEffort),
	}
}

// NewServerWithThread wires a server for tests that supply a custom Thread client.
func NewServerWithThread(port int, threads *thread.Client, mockMode bool, stateDir string, status runtime.Reporter) *Server {
	tracker, _ := status.(*runtime.Tracker)
	e := &env.Env{
		Config: &config.Config{
			Runtime:  config.RuntimeModeFromMock(mockMode),
			StateDir: stateDir,
			Port:     port,
		},
		Status: tracker,
		Thread: threads,
	}
	return NewServer(e, port, stateDir)
}

// NewServerWithOtCtl is a convenience constructor for tests and wiring.
func NewServerWithOtCtl(port int, runner otctl.Runner, mockMode bool) *Server {
	return NewServer(testEnv(runner, mockMode), port, "")
}
