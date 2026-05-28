// Package main is the entry point for the ThreadGate Standalone OTBR Orchestrator.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/api"
	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
	"github.com/MikeO7/threadgate/src/manager/internal/supervisor"
)

func main() {
	log.Println("====================================================")
	log.Println("      ThreadGate Standalone OTBR Orchestrator       ")
	log.Println("====================================================")

	cfg := config.Load()

	// 1. Audit Host Environment Check on startup
	hostAudit := hardware.AuditHost()
	log.Printf("[Main] Host Audit completed. %d warnings found.\n", len(hostAudit.Warnings))
	for _, w := range hostAudit.Warnings {
		log.Printf("[Main] Warning: %s\n", w)
	}

	radioURL := resolveRadioURL(cfg)
	log.Printf("[Main] Using Thread Radio URL: %s\n", radioURL)

	probedVersion, devicePath, probeErr := runRadioProbe(cfg, radioURL)
	probeErrorStr := ""
	if probeErr != nil {
		probeErrorStr = probeErr.Error()
	}
	hardware.SetHealth(hardware.HealthStatus{
		HostAudit:     hostAudit,
		ProbedVersion: probedVersion,
		ProbeError:    probeErrorStr,
		RadioPath:     devicePath,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	super := supervisor.New(radioURL, cfg.StateDir, cfg.LogLevel, cfg.Runtime, cfg.BackboneIF)
	if err := super.Start(ctx); err != nil {
		log.Printf("[Main] Supervisor boot failed: %v\n", err)
		return
	}

	server := startAPIServer(cfg)
	waitForShutdown(server, super, cancel)
}

func startAPIServer(cfg *config.Config) *api.Server {
	otctl := api.NewOtCtl(cfg.MockMode)
	threads := api.NewThreadService(otctl, api.CollectBestEffort)
	server := api.NewServer(cfg.Port, threads, cfg.MockMode, cfg.StateDir)
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("[Main] API/Web Server stopped or failed: %v\n", err)
		}
	}()
	return server
}

func waitForShutdown(server *api.Server, super *supervisor.Supervisor, cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("[Main] Received system signal %v. Cleaning up...\n", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Main] API server shutdown error: %v\n", err)
	}

	super.Stop()
	cancel()

	log.Println("[Main] ThreadGate Orchestrator shutdown successfully completed.")
}

func resolveRadioURL(cfg *config.Config) string {
	if cfg.RadioURL != "" {
		return cfg.RadioURL
	}
	if !cfg.AutoDiscover {
		log.Fatalf("[Main] Configuration error: OTBR_RADIO_URL is not set and auto-discovery is disabled.")
	}

	discoveredPath, err := hardware.DiscoverRadio(cfg.Runtime.IsMock())
	if err != nil {
		log.Fatalf("[Main] Configuration error: %v. Please set OTBR_RADIO_URL environment variable explicitly.", err)
	}

	flowParam := ""
	if cfg.FlowControl {
		flowParam = "&uart-flow-control"
	}
	return fmt.Sprintf("spinel+hdlc+uart://%s?uart-baudrate=%d%s", discoveredPath, cfg.Baudrate, flowParam)
}

func runRadioProbe(cfg *config.Config, radioURL string) (probedVersion, devicePath string, probeErr error) {
	devicePath, baudrate := parseSerialFromURL(radioURL, cfg.Baudrate)

	switch {
	case cfg.Runtime.IsMock():
		probedVersion = "ThreadGateMock/1.0.0; SIMULATION; May 28 2026"
		log.Printf("[Main] Mock mode active: skipping hardware probe. Probed version set to simulated: %s\n", probedVersion)
	case devicePath != "":
		log.Printf("[Main] Probing physical radio %s at %d baud...\n", devicePath, baudrate)
		probedVersion, probeErr = hardware.ProbeDevice(devicePath, baudrate)
		if probeErr != nil {
			log.Printf("[Main] Pre-flight radio probe failed: %v\n", probeErr)
		} else {
			log.Printf("[Main] Pre-flight radio probe succeeded. RCP Version: %s\n", probedVersion)
		}
	default:
		log.Println("[Main] Network-based or non-serial RCP detected. Skipping serial pre-flight hardware probe.")
	}

	return probedVersion, devicePath, probeErr
}

func parseSerialFromURL(radioURL string, defaultBaud int) (string, int) {
	prefix := "spinel+hdlc+uart://"
	if !strings.HasPrefix(radioURL, prefix) {
		return "", defaultBaud
	}
	rawPath := strings.TrimPrefix(radioURL, prefix)

	parts := strings.Split(rawPath, "?")
	devicePath := parts[0]
	if len(parts) <= 1 {
		return devicePath, defaultBaud
	}

	baud := defaultBaud
	for _, param := range strings.Split(parts[1], "&") {
		if !strings.HasPrefix(param, "uart-baudrate=") {
			continue
		}
		val := strings.TrimPrefix(param, "uart-baudrate=")
		if b, err := strconv.Atoi(val); err == nil && b > 0 {
			baud = b
		}
	}
	return devicePath, baud
}
