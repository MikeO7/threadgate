package hassdev

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SetupOptions controls a full Home Assistant bootstrap.
type SetupOptions struct {
	SeedDevices   bool
	ConfigureOTBR bool
	UseFixture    bool
	BuildFixture  bool
}

// Setup waits for HA, runs onboarding, optional seed/OTBR, and writes credentials.
func Setup(ctx context.Context, cfg Config, opts SetupOptions) error {
	if err := WaitHomeAssistant(ctx, cfg, 5*time.Minute); err != nil {
		return fmt.Errorf("home assistant not ready: %w", err)
	}

	creds, _ := LoadCredentials(cfg.CredsFile)
	if err := runOnboardingIfNeeded(ctx, cfg, &creds); err != nil {
		return err
	}

	creds, err := LoadCredentials(cfg.CredsFile)
	if err != nil {
		return err
	}
	if err := applySetupOptions(ctx, cfg, opts, creds); err != nil {
		return err
	}
	return ensureCoreConfigWithToken(ctx, cfg, creds)
}

func runOnboardingIfNeeded(ctx context.Context, cfg Config, creds *Credentials) error {
	if shouldSkipOnboarding(ctx, cfg, *creds) {
		_, _ = fmt.Fprintln(os.Stdout, "==> Existing credentials valid; skipping onboarding")
		return nil
	}
	if err := RunOnboarding(ctx, cfg, creds); err != nil {
		return err
	}
	return persistLongLivedToken(ctx, cfg, creds)
}

func shouldSkipOnboarding(ctx context.Context, cfg Config, creds Credentials) bool {
	if creds.HALongLivedToken == "" {
		return false
	}
	if err := VerifyToken(ctx, cfg, creds.HALongLivedToken); err != nil {
		return false
	}
	steps, err := newHTTPClient(cfg).onboardingStatus(ctx)
	return err == nil && onboardingComplete(steps)
}

func persistLongLivedToken(ctx context.Context, cfg Config, creds *Credentials) error {
	_, _ = fmt.Fprintln(os.Stdout, "==> Creating long-lived access token")
	llt, err := createLongLivedToken(ctx, cfg.HAURL, creds.HAAccessToken, "threadgate", 3650)
	if err != nil {
		return err
	}
	creds.HALongLivedToken = llt
	if err := creds.Save(cfg.CredsFile); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Wrote %s\n", cfg.CredsFile)
	return nil
}

func applySetupOptions(ctx context.Context, cfg Config, opts SetupOptions, creds Credentials) error {
	if opts.SeedDevices {
		if err := seedMockDevices(cfg); err != nil {
			return err
		}
	}
	if !opts.ConfigureOTBR {
		return nil
	}
	if err := EnsureOTBRIntegration(ctx, cfg, creds.HALongLivedToken); err != nil {
		_, _ = fmt.Fprintf(os.Stdout, "==> OTBR configure warning: %v\n", err)
	}
	return nil
}

func seedMockDevices(cfg Config) error {
	_, _ = fmt.Fprintln(os.Stdout, "==> Seeding mock Thread devices (stop HA first)")
	created, err := SeedDeviceRegistry(cfg.HAConfigDir)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Seeded %d device(s) into device registry storage\n", created)
	return nil
}

func ensureCoreConfigWithToken(ctx context.Context, cfg Config, creds Credentials) error {
	token := creds.HALongLivedToken
	if token == "" {
		token = creds.HAAccessToken
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Ensuring Home Assistant location (%s)\n", cfg.HACountry)
	if err := EnsureCoreConfig(ctx, cfg, token); err != nil {
		return fmt.Errorf("core config: %w", err)
	}
	return nil
}

// SetupFromFixture applies tarball, then waits for Home Assistant (must already be starting).
func SetupFromFixture(ctx context.Context, cfg Config) error {
	if err := ApplyFixture(cfg); err != nil {
		return err
	}
	if _, err := LoadCredentials(cfg.CredsFile); err != nil {
		return err
	}
	return WaitHomeAssistant(ctx, cfg, 5*time.Minute)
}

// RemovePairingState clears ThreadGate-side HASS pairing file.
func RemovePairingState(cfg Config) error {
	path := filepath.Join(cfg.Root, "data", "hass_config.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
