// Command hassdev configures Home Assistant for local ThreadGate integration testing.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/hassdev"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	root, _ := os.Getwd()
	cfg := hassdev.DefaultConfig(root)
	ctx := context.Background()

	fn, ok := commands[os.Args[1]]
	if !ok {
		usage()
		os.Exit(2)
	}
	fn(ctx, cfg, os.Args[2:])
}

var commands = map[string]func(context.Context, hassdev.Config, []string){
	"setup":          runSetup,
	"pair":           runPair,
	"pair-initiate":  runPairInitiate,
	"test":           runTest,
	"fixture-build":  runFixtureBuild,
	"fixture-apply":  runFixtureApply,
	"wait-ha":        runWaitHA,
	"wait-tg":        runWaitTG,
	"seed-devices":   runSeedDevices,
	"ensure-otbr":    runEnsureOTBR,
	"repair-otbr":    runRepairOTBR,
	"verify":         runVerify,
	"ensure-core":    runEnsureCore,
}

func runSetup(ctx context.Context, cfg hassdev.Config, args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	seed := fs.Bool("seed-devices", false, "seed mock Thread devices (stop HA first)")
	otbr := fs.Bool("configure-otbr", false, "add OTBR integration in HA")
	_ = fs.Parse(args)
	if err := hassdev.Setup(ctx, cfg, hassdev.SetupOptions{
		SeedDevices:   *seed,
		ConfigureOTBR: *otbr,
	}); err != nil {
		fatal(err)
	}
}

func runPair(ctx context.Context, cfg hassdev.Config, args []string) {
	fs := flag.NewFlagSet("pair", flag.ExitOnError)
	auto := fs.Bool("auto-approve", false, "approve via API (skips dashboard popup)")
	force := fs.Bool("force", false, "clear data/hass_config.json before pairing")
	_ = fs.Parse(args)
	if err := hassdev.PairThreadGate(ctx, cfg, *auto, *force); err != nil {
		fatal(err)
	}
}

func runPairInitiate(ctx context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.InitiatePairingOnly(ctx, cfg); err != nil {
		fatal(err)
	}
}

func runTest(ctx context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.RunSmokeTests(ctx, cfg); err != nil {
		fatal(err)
	}
	_, _ = fmt.Fprintln(os.Stdout, "\nAll smoke tests passed")
}

func runFixtureBuild(_ context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.BuildFixture(cfg); err != nil {
		fatal(err)
	}
}

func runFixtureApply(_ context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.ApplyFixture(cfg); err != nil {
		fatal(err)
	}
	if _, err := hassdev.LoadCredentials(cfg.CredsFile); err != nil {
		fatal(err)
	}
	_, _ = fmt.Fprintln(os.Stdout, "==> Fixture applied (start Home Assistant next)")
}

func runWaitHA(ctx context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.WaitHomeAssistant(ctx, cfg, 5*time.Minute); err != nil {
		fatal(err)
	}
}

func runWaitTG(ctx context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.WaitURL(ctx, cfg.TGURL+"/api/health", 90*time.Second); err != nil {
		fatal(err)
	}
}

func runSeedDevices(_ context.Context, cfg hassdev.Config, _ []string) {
	n, err := hassdev.SeedDeviceRegistry(cfg.HAConfigDir)
	if err != nil {
		fatal(err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Seeded %d device(s)\n", n)
}

func runEnsureOTBR(ctx context.Context, cfg hassdev.Config, _ []string) {
	creds, err := hassdev.LoadCredentials(cfg.CredsFile)
	if err != nil {
		fatal(err)
	}
	if err := hassdev.EnsureOTBRIntegration(ctx, cfg, creds.HALongLivedToken); err != nil {
		fatal(err)
	}
}

func runRepairOTBR(ctx context.Context, cfg hassdev.Config, _ []string) {
	creds, err := hassdev.LoadCredentials(cfg.CredsFile)
	if err != nil {
		fatal(err)
	}
	if err := hassdev.RepairOTBRIntegration(ctx, cfg, creds.HALongLivedToken); err != nil {
		fatal(err)
	}
}

func runVerify(ctx context.Context, cfg hassdev.Config, _ []string) {
	if err := hassdev.VerifyThreadGateHAAPI(ctx, cfg); err != nil {
		fatal(err)
	}
	_, _ = fmt.Fprintln(os.Stdout, "==> ThreadGate ↔ Home Assistant wiring OK")
}

func runEnsureCore(ctx context.Context, cfg hassdev.Config, _ []string) {
	creds, err := hassdev.LoadCredentials(cfg.CredsFile)
	if err != nil {
		fatal(err)
	}
	token := creds.HALongLivedToken
	if token == "" {
		token = creds.HAAccessToken
	}
	if err := hassdev.EnsureCoreConfig(ctx, cfg, token); err != nil {
		fatal(err)
	}
	_, _ = fmt.Fprintf(os.Stdout, "==> Home Assistant location set (%s)\n", cfg.HACountry)
}

func usage() {
	_, _ = fmt.Fprintf(os.Stderr, `Usage: hassdev <command>

Commands:
  setup [--seed-devices] [--configure-otbr]
  pair [--auto-approve] [--force]   Link HA to ThreadGate (default: dashboard banner)
  pair-initiate                     Create pending request only (open dashboard to approve)
  test
  fixture-build
  fixture-apply
  wait-ha
  wait-tg
  seed-devices       Patch HA device registry (HA must be stopped)
  ensure-core        Set country/location (default US) via WebSocket
  ensure-otbr        Add or repair HA Open Thread Border Router → ThreadGate URL
  repair-otbr        Reload (or recreate) a failed OTBR config entry
  verify             Check pairing, OTBR loaded, dataset, and REST API

Example:
  go run ./src/manager/cmd/hassdev setup --configure-otbr
`)
}

func fatal(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
