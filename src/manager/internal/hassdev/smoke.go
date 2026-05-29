package hassdev

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunSmokeTests verifies HA ↔ ThreadGate connectivity.
func RunSmokeTests(ctx context.Context, cfg Config) error {
	creds, err := LoadCredentials(cfg.CredsFile)
	if err != nil {
		return fmt.Errorf("load credentials: %w", err)
	}
	if err := waitForSmokeEndpoints(ctx, cfg); err != nil {
		return err
	}
	if err := smokeHATemplateAPI(ctx, cfg, creds.HALongLivedToken); err != nil {
		return err
	}
	if err := smokeHAContainerToThreadGate(ctx); err != nil {
		return err
	}
	if err := smokePairingFile(cfg); err != nil {
		return err
	}
	if err := smokeTopologyOrRegistry(ctx, cfg, creds.HALongLivedToken); err != nil {
		return err
	}
	if err := smokeDockerNetworkHA(ctx, creds.HALongLivedToken); err != nil {
		return err
	}
	if err := VerifyThreadGateHAAPI(ctx, cfg); err != nil {
		return err
	}
	return smokeOTBRIntegration(ctx, cfg, creds.HALongLivedToken)
}

func waitForSmokeEndpoints(ctx context.Context, cfg Config) error {
	if err := WaitURL(ctx, cfg.TGURL+"/api/health", 60*time.Second); err != nil {
		return fmt.Errorf("threadgate: %w", err)
	}
	if err := WaitURL(ctx, cfg.HAURL+"/", 60*time.Second); err != nil {
		return fmt.Errorf("home assistant: %w", err)
	}
	return nil
}

func smokeHATemplateAPI(ctx context.Context, cfg Config, token string) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	templateBody, _ := json.Marshal(map[string]string{"template": "ok"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.HAURL+"/api/template", bytes.NewReader(templateBody))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ha template: %s", string(body))
	}
	_, _ = fmt.Fprintf(os.Stdout, "PASS: HA template API (%s)\n", strings.TrimSpace(string(body)))
	return nil
}

func smokeHAContainerToThreadGate(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "exec", "threadgate-ha", "curl", "-sf", "http://threadgate:8081/api/node")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ha container -> threadgate: %w: %s", err, strings.TrimSpace(string(out)))
	}
	_, _ = fmt.Fprintln(os.Stdout, "PASS: HA container → ThreadGate OTBR API")
	return nil
}

func smokePairingFile(cfg Config) error {
	pairPath := cfg.Root + "/data/hass_config.json"
	if _, err := os.Stat(pairPath); err != nil {
		return fmt.Errorf("missing %s (run hassdev pair)", pairPath)
	}
	_, _ = fmt.Fprintln(os.Stdout, "PASS: ThreadGate hass_config.json present")
	return nil
}

func smokeTopologyOrRegistry(ctx context.Context, cfg Config, token string) error {
	count, err := topologyDeviceNameCount(ctx, cfg)
	if err != nil {
		return err
	}
	if count >= 5 {
		_, _ = fmt.Fprintf(os.Stdout, "PASS: topology deviceNames from Home Assistant (%d entries)\n", count)
		return nil
	}
	return smokeDeviceRegistryFallback(ctx, cfg, token)
}

func topologyDeviceNameCount(ctx context.Context, cfg Config) (int, error) {
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	topoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.TGURL+"/api/topology", nil)
	if err != nil {
		return 0, err
	}
	topoResp, err := client.Do(topoReq)
	if err != nil {
		return 0, err
	}
	defer func() { _ = topoResp.Body.Close() }()
	topoData, err := io.ReadAll(topoResp.Body)
	if err != nil {
		return 0, err
	}
	var topo struct {
		DeviceNames map[string]string `json:"deviceNames"`
	}
	if err := json.Unmarshal(topoData, &topo); err != nil {
		return 0, err
	}
	return len(topo.DeviceNames), nil
}

func smokeDeviceRegistryFallback(ctx context.Context, cfg Config, token string) error {
	n, err := countHADevicesWS(ctx, cfg, token)
	if err != nil {
		return fmt.Errorf("topology deviceNames empty and registry list failed: %w", err)
	}
	if n < 3 {
		return fmt.Errorf("expected >=3 devices in HA registry, got %d", n)
	}
	_, _ = fmt.Fprintf(os.Stdout, "PASS: Home Assistant device registry has %d devices\n", n)
	return nil
}

func smokeDockerNetworkHA(ctx context.Context, token string) error {
	cmd := exec.CommandContext(ctx, "docker", "exec", "-i", "threadgate-ha", "curl", "-sf", "-X", "POST",
		"http://homeassistant:8123/api/template",
		"-H", "@-",
		"-H", "Content-Type: application/json",
		"-d", `{"template":"ok"}`)
	cmd.Stdin = strings.NewReader("Authorization: Bearer " + token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker network HA API: %w: %s", err, strings.TrimSpace(string(out)))
	}
	_, _ = fmt.Fprintln(os.Stdout, "PASS: Docker network → Home Assistant API (ThreadGate uses same path)")
	return nil
}

func smokeOTBRIntegration(ctx context.Context, cfg Config, token string) error {
	entry, err := findOTBREntry(ctx, cfg, token)
	if err != nil {
		if errors.Is(err, errOTBRNotFound) {
			return fmt.Errorf("home assistant has no OTBR integration")
		}
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "PASS: HA OTBR integration loaded (entry %s)\n", entry.EntryID)
	_, _ = fmt.Fprintln(os.Stdout, "PASS: ThreadGate OTBR API + pairing file + active dataset")
	return nil
}
