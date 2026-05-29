package hassdev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

// EnsureOTBRIntegration adds or repairs the HA Open Thread Border Router integration.
func EnsureOTBRIntegration(ctx context.Context, cfg Config, token string) error {
	entry, err := findOTBREntry(ctx, cfg, token)
	if err != nil && !errors.Is(err, errOTBRNotFound) {
		return err
	}
	if entry != nil {
		if entry.State == haEntryStateLoaded {
			_, _ = fmt.Fprintln(os.Stdout, "==> Home Assistant OTBR integration already configured")
			return nil
		}
		return RepairOTBRIntegration(ctx, cfg, token)
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Configuring OTBR integration (%s)\n", cfg.OTBRURL)
	result, err := configureOTBR(ctx, cfg, token, cfg.OTBRURL)
	if err != nil {
		return err
	}
	if result != "create_entry" {
		return fmt.Errorf("OTBR flow did not complete (result=%q)", result)
	}
	_, _ = fmt.Fprintln(os.Stdout, "==> OTBR integration added in Home Assistant")
	return RepairOTBRIntegration(ctx, cfg, token)
}

// VerifyThreadGateHAAPI checks OTBR + HASS wiring end-to-end.
func VerifyThreadGateHAAPI(ctx context.Context, cfg Config) error {
	creds, err := LoadCredentials(cfg.CredsFile)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	if err := verifyThreadGateNodeAPI(ctx, client, cfg); err != nil {
		return err
	}
	if err := verifyOTBREntryLoaded(ctx, cfg, creds.HALongLivedToken); err != nil {
		return err
	}
	if err := verifyPairingFilePresent(cfg); err != nil {
		return err
	}
	return verifyTopologyEndpoint(ctx, client, cfg)
}

func verifyThreadGateNodeAPI(ctx context.Context, client *http.Client, cfg Config) error {
	if err := verifyHTTPGetOK(ctx, client, strings.TrimSuffix(cfg.TGURL, "/")+"/node/ba-id", "threadgate /node/ba-id"); err != nil {
		return err
	}
	dsBody, err := fetchHTTPBody(ctx, client, strings.TrimSuffix(cfg.TGURL, "/")+"/node/dataset/active")
	if err != nil {
		return fmt.Errorf("threadgate /node/dataset/active: %w", err)
	}
	hexDS := strings.TrimSpace(string(dsBody))
	if len(hexDS) < 40 {
		return fmt.Errorf("threadgate active dataset too short (%d chars)", len(hexDS))
	}
	if thread.DatasetContainsInsecureNetworkKey(hexDS) {
		return fmt.Errorf("threadgate active dataset uses HA-flagged default network key — rebuild ThreadGate or form a new Thread network")
	}
	return nil
}

func verifyHTTPGetOK(ctx context.Context, client *http.Client, url, label string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", label, string(body))
	}
	return nil
}

func fetchHTTPBody(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}
	return body, readErr
}

func verifyOTBREntryLoaded(ctx context.Context, cfg Config, token string) error {
	entry, err := findOTBREntry(ctx, cfg, token)
	if err != nil {
		if errors.Is(err, errOTBRNotFound) {
			return fmt.Errorf("home assistant has no OTBR integration — run: hassdev ensure-otbr")
		}
		return err
	}
	if entry.State != haEntryStateLoaded {
		return fmt.Errorf("home assistant OTBR entry %s state=%q (want loaded) — run: hassdev repair-otbr", entry.EntryID, entry.State)
	}
	return nil
}

func verifyPairingFilePresent(cfg Config) error {
	if _, err := os.Stat(hassConfigPath(cfg)); err != nil {
		return fmt.Errorf("ThreadGate not paired (missing %s)", hassConfigPath(cfg))
	}
	return nil
}

func verifyTopologyEndpoint(ctx context.Context, client *http.Client, cfg Config) error {
	topoReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(cfg.TGURL, "/")+"/api/topology", nil)
	if err != nil {
		return err
	}
	topoResp, err := client.Do(topoReq)
	if err != nil {
		return err
	}
	defer func() { _ = topoResp.Body.Close() }()
	topoBody, _ := io.ReadAll(topoResp.Body)
	if topoResp.StatusCode != http.StatusOK {
		return fmt.Errorf("topology: %s", string(topoBody))
	}
	return nil
}
