package hardware

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditHostSafety(t *testing.T) {
	_ = t
	audit := AuditHost()
	_ = audit.Warnings
}

func TestCheckSysctl(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "forwarding")
	if err := os.WriteFile(path, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !checkSysctl(path, "1") {
		t.Error("expected matching sysctl value")
	}
	if checkSysctl(path, "0") {
		t.Error("expected non-matching sysctl value")
	}
	if checkSysctl(filepath.Join(dir, "missing"), "1") {
		t.Error("expected missing file to return false")
	}
}
