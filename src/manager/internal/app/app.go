// Package app coordinates ThreadGate startup, shutdown, and the HTTP API lifecycle.
package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/api"
	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/env"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/supervisor"
)

// App coordinates the lifecycle, background daemons, and HTTP interface of ThreadGate.
type App struct {
	cfg *config.Config
}

var (
	waitForShutdownHook = waitForShutdown
	fatalLog            = log.Fatalf
)

// New creates a new ThreadGate application instance.
func New() *App {
	return &App{
		cfg: config.Load(),
	}
}

// Run executes the application, starting all systems and coordinating their shutdown.
func (a *App) Run() error {
	log.Println("====================================================")
	log.Println("      ThreadGate Standalone OTBR Orchestrator       ")
	log.Println("====================================================")

	hostAudit := hardware.AuditHost()
	log.Printf("[App] Host Audit completed. %d warnings found.\n", len(hostAudit.Warnings))
	for _, w := range hostAudit.Warnings {
		log.Printf("[App] Warning: %s\n", w)
	}

	originalPort := a.cfg.Port
	a.cfg.Port = findAvailablePort(a.cfg.Port)
	if a.cfg.Port != originalPort {
		log.Printf("[App] WARNING: Port %d was already in use! Auto-detected free port %d instead for the dashboard/API.\n", originalPort, a.cfg.Port)
	}

	runtimeEnv, err := env.Bootstrap(a.cfg)
	if err != nil {
		return fmt.Errorf("bootstrap failed: %w", err)
	}
	runtimeEnv.Status.SetHostAudit(hostAudit)

	log.Printf("[App] Using Thread Radio URL: %s\n", runtimeEnv.Radio.CurrentSpinelURL())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	super := supervisor.New(runtimeEnv, supervisor.ExecLauncher{})
	if err := super.Start(ctx); err != nil {
		return fmt.Errorf("supervisor boot failed: %w", err)
	}

	server, apiErrChan := startAPIServer(runtimeEnv)
	waitForShutdownHook(server, super, cancel, apiErrChan, nil)

	return nil
}

func findAvailablePort(startPort int) int {
	lc := net.ListenConfig{}
	for port := startPort; port < startPort+100; port++ {
		ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = ln.Close()
			return port
		}
	}
	return startPort
}

func startAPIServer(runtimeEnv *env.Env) (*api.Server, <-chan error) {
	server := api.NewServer(runtimeEnv, runtimeEnv.Config.Port, runtimeEnv.Config.StateDir)
	errChan := make(chan error, 1)
	go func() {
		errChan <- server.Start()
	}()
	return server, errChan
}

func waitForShutdown(server *api.Server, super *supervisor.Supervisor, cancel context.CancelFunc, apiErrChan <-chan error, sigChan chan os.Signal) {
	if sigChan == nil {
		sigChan = make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	}

	select {
	case sig := <-sigChan:
		log.Printf("[App] Received signal %v, shutting down...\n", sig)
	case err := <-apiErrChan:
		if err != nil && err != http.ErrServerClosed {
			fatalLog("[App] API server failed: %v\n", err)
		}
	}

	cancel()
	super.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[App] API shutdown error: %v\n", err)
	}
	log.Println("[App] Shutdown complete.")
}
