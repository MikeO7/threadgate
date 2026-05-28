package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
	"github.com/MikeO7/threadgate/src/manager/internal/hardware"
)

func TestRunAgentLoop(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		otbrAgentName: exitZeroScript,
	})
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	cfg := mockConfig(false)
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)

	go s.runAgentLoop()
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestRunAgentOnceDiscoveryFailure(t *testing.T) {
	binDir := writeFakeCommands(t, map[string]string{
		otbrAgentName: exitZeroScript,
	})
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+oldPath)

	root := t.TempDir()
	restore := hardware.SetDiscoveryPathsForTest(
		filepath.Join(root, "dev"),
		filepath.Join(root, "serial", "by-id"),
		filepath.Join(root, "sys", "bus", "usb", "devices"),
	)
	t.Cleanup(restore)

	cfg := mockConfig(true)
	cfg.RadioURL = "spinel+hdlc+uart:///dev/ttyFALLBACK0"
	cfg.Runtime = config.RuntimeModeHardware
	s := newTestSupervisor(t, cfg, nil, fakeLauncher{binDir: binDir})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx, s.cancelFunc = context.WithCancel(ctx)
	s.runAgentOnce()
	cancel()
}
