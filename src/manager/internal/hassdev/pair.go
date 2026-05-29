package hassdev

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/pairing"
)

func hassConfigPath(cfg Config) string {
	return filepath.Join(cfg.Root, "data", "hass_config.json")
}

// IsThreadGatePaired reports whether ThreadGate already has saved HA credentials.
func IsThreadGatePaired(cfg Config) bool {
	_, err := os.Stat(hassConfigPath(cfg))
	return err == nil
}

func pairClient(cfg Config) *pairing.HTTPClient {
	return &pairing.HTTPClient{
		BaseURL: cfg.TGURL,
		HTTPClient: &http.Client{
			Timeout: cfg.HTTPTimeout,
		},
	}
}

// InitiatePairing starts a pending pairing request on the ThreadGate dashboard.
func InitiatePairing(ctx context.Context, cfg Config) (string, error) {
	creds, err := LoadCredentials(cfg.CredsFile)
	if err != nil {
		return "", err
	}
	pairURL := cfg.HAPairURL
	if pairURL == "" {
		pairURL = cfg.HAURL
	}
	return pairClient(cfg).Initiate(ctx, "Home Assistant", pairURL, creds.HALongLivedToken)
}

// WaitForPairingApproval blocks until the user approves in the ThreadGate dashboard or timeout.
func WaitForPairingApproval(ctx context.Context, cfg Config, pairingID string, timeout time.Duration) error {
	return pairClient(cfg).WaitForApproval(ctx, pairingID, timeout)
}

// InitiatePairingOnly starts a pending request and prints dashboard instructions (no wait).
func InitiatePairingOnly(ctx context.Context, cfg Config) error {
	pairingID, err := InitiatePairing(ctx, cfg)
	if err != nil {
		return err
	}
	dashboard := strings.TrimSuffix(cfg.TGURL, "/")
	_, _ = fmt.Fprintf(os.Stdout, "==> Pairing request %s pending\n", pairingID)
	_, _ = fmt.Fprintf(os.Stdout, "==> Open %s and click **Approve Connection** on the banner\n", dashboard)
	return nil
}

// PairThreadGate links Home Assistant to ThreadGate.
// When autoApprove is false, the user must approve via the dashboard popup at TGURL.
func PairThreadGate(ctx context.Context, cfg Config, autoApprove, force bool) error {
	if IsThreadGatePaired(cfg) && autoApprove && !force {
		_, _ = fmt.Fprintln(os.Stdout, "==> ThreadGate already paired with Home Assistant")
		return nil
	}
	if force {
		if err := RemovePairingState(cfg); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(os.Stdout, "==> Cleared previous ThreadGate pairing state")
	}

	pairingID, err := InitiatePairing(ctx, cfg)
	if err != nil {
		return err
	}

	if autoApprove {
		if err := pairClient(cfg).Approve(ctx, pairingID); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, "==> Pairing approved (%s)\n", pairingID)
		return nil
	}

	dashboard := strings.TrimSuffix(cfg.TGURL, "/")
	_, _ = fmt.Fprintf(os.Stdout, "==> Pairing request %s created\n", pairingID)
	_, _ = fmt.Fprintf(os.Stdout, "==> Open %s — approve the **Secure Connection Request** banner\n", dashboard)
	_, _ = fmt.Fprintln(os.Stdout, "    (waiting up to 10 minutes…)")

	if err := WaitForPairingApproval(ctx, cfg, pairingID, 10*time.Minute); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Pairing approved (%s)\n", pairingID)
	return nil
}
