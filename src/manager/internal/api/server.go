// Package api exposes REST APIs and sleek monitoring dashboards for the ThreadGate ecosystem.
package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/hass"
	"github.com/MikeO7/threadgate/src/manager/internal/otbrapi"
	"github.com/MikeO7/threadgate/src/manager/internal/pairing"
	"github.com/MikeO7/threadgate/src/manager/internal/radio"
	"github.com/MikeO7/threadgate/src/manager/internal/runtime"
	"github.com/MikeO7/threadgate/src/manager/internal/snapshot"
	"github.com/MikeO7/threadgate/src/manager/internal/thread"
	"github.com/MikeO7/threadgate/src/manager/internal/topology"
)

// Server handles REST API and diagnostic web requests.
type Server struct {
	port           int
	env            *env.Env
	threads        *thread.Client
	backup         *BackupStore
	mu             sync.Mutex
	srv            *http.Server
	statusReporter runtime.Reporter
	hassClient     *hass.Client
	pairMgr        *pairing.Manager
	snapSvc        *snapshot.Service
	otbr           *otbrapi.Handler
	readiness      *radio.Readiness
}

// NewServer creates a server wired to the composition root Env.
func NewServer(e *env.Env, port int, stateDir string) *Server {
	var reporter runtime.Reporter
	if e.Status != nil {
		reporter = e.Status
	}
	hassClient := hass.NewClient(e.Config)
	pairMgr := pairing.NewManager()
	s := &Server{
		port:           port,
		env:            e,
		threads:        e.Thread,
		backup:         NewBackupStore(e.Thread, stateDir),
		statusReporter: reporter,
		hassClient:     hassClient,
		pairMgr:        pairMgr,
		snapSvc: &snapshot.Service{
			Threads: e.Thread,
			Hass:    hassClient,
			Pairing: pairMgr,
		},
		readiness: radio.NewReadiness(e.Config, e.Status),
	}
	s.otbr = &otbrapi.Handler{
		Ops:      &otbrapi.ClientAdapter{Client: e.Thread},
		Status:   reporter,
		MockMode: e.IsMock(),
	}
	return s
}

// Handler returns the HTTP handler for testing without binding a port.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return s.LoggingMiddleware(mux)
}

// Start launches the HTTP listener.
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()

	log.Printf("[API Server] Exposing REST API and modern dashboard on port %d...\n", s.port)
	return srv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP listener.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// RunSnapshot is a test helper that builds a snapshot with a timeout context.
func RunSnapshot(ctx context.Context, threads *thread.Client) (topology.Snapshot, error) {
	return threads.BuildSnapshot(ctx)
}
