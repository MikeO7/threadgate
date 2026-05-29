package hassdev

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const haContainerName = "threadgate-ha"

var errHANotReady = fmt.Errorf("home assistant not ready")

// WaitHomeAssistant waits for the HA container to be running and serving HTTP.
func WaitHomeAssistant(ctx context.Context, cfg Config, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastLog time.Time

	for time.Now().Before(deadline) {
		if err := waitHomeAssistantOnce(ctx, cfg, &lastLog); err == nil {
			return nil
		}
		if err := sleepOrDone(ctx, 2*time.Second); err != nil {
			return err
		}
	}
	return waitHomeAssistantTimeout(cfg)
}

func waitHomeAssistantOnce(ctx context.Context, cfg Config, lastLog *time.Time) error {
	running, err := haContainerRunning(ctx)
	if err != nil {
		return err
	}
	if !running {
		maybeLogWait(lastLog, fmt.Sprintf("==> Waiting for %s container to start...\n", haContainerName))
		return errHANotReady
	}
	if err := WaitURL(ctx, cfg.HAURL+"/", 8*time.Second); err == nil {
		_, _ = fmt.Fprintln(os.Stdout, "==> Home Assistant is ready")
		return nil
	}
	maybeLogWait(lastLog, "==> Home Assistant container up; waiting for http://127.0.0.1:8123 ...\n")
	return errHANotReady
}

func maybeLogWait(lastLog *time.Time, msg string) {
	if time.Since(*lastLog) <= 10*time.Second {
		return
	}
	_, _ = fmt.Fprint(os.Stdout, msg)
	*lastLog = time.Now()
}

func waitHomeAssistantTimeout(cfg Config) error {
	running, _ := haContainerRunning(context.Background())
	if !running {
		return fmt.Errorf("timeout: %s is not running (try: docker compose -f docker-compose.integration.yml up -d homeassistant)", haContainerName)
	}
	return fmt.Errorf("timeout waiting for %s (container running but port not ready)", cfg.HAURL)
}

func sleepOrDone(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func haContainerRunning(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", haContainerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out) + " " + err.Error())
		if strings.Contains(msg, "No such object") {
			return false, nil
		}
		return false, fmt.Errorf("docker inspect %s: %s", haContainerName, msg)
	}
	return strings.TrimSpace(string(out)) == "true", nil
}
