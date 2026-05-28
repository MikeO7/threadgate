package api

import (
	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

// OtCtl is the seam for local ot-ctl execution (exec adapter in prod, mock adapter in tests).
type OtCtl = otctl.Runner

// ThreadService is the deep Thread client module (kept as alias for existing call sites).
type ThreadService = thread.Client

// CollectMode controls how ot-ctl collection failures are handled.
type CollectMode = thread.Policy

const (
	CollectBestEffort = thread.PolicyBestEffort
	CollectStrict     = thread.PolicyStrict
)

// FuncOtCtl adapts a function to the OtCtl interface (for tests).
type FuncOtCtl = thread.FuncRunner

// NewThreadService wires a Thread client with the given ot-ctl adapter.
func NewThreadService(runner OtCtl, collectMode CollectMode) *ThreadService {
	return thread.NewClient(runner, collectMode)
}

// NewOtCtl returns the production or mock adapter for the given mode.
func NewOtCtl(mockMode bool) OtCtl {
	return thread.NewRunner(config.RuntimeModeFromMock(mockMode))
}

// NewMockOtCtl returns a mock ot-ctl adapter with default simulated Thread data.
func NewMockOtCtl() *thread.Mock {
	return thread.NewMock()
}

// testEnv builds a minimal Env for API tests without full bootstrap.
func testEnv(runner OtCtl, mockMode bool) *env.Env {
	return testEnvWithStatus(runner, mockMode, nil)
}

// NewServerWithThread wires a server for tests that supply a custom Thread client.
func NewServerWithThread(port int, threads *ThreadService, mockMode bool, stateDir string, status runtime.Reporter) *Server {
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

func testEnvWithStatus(runner OtCtl, mockMode bool, status *runtime.Tracker) *env.Env {
	return &env.Env{
		Config: &config.Config{Runtime: config.RuntimeModeFromMock(mockMode)},
		Status: status,
		Thread: thread.NewClient(runner, thread.PolicyBestEffort),
	}
}
