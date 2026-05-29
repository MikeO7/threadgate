package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/threadgate/src/manager/internal/config"
)

func TestEnsureStatePersistence_MockMode(t *testing.T) {
	cfg := &config.Config{
		Runtime:  config.RuntimeModeMock,
		StateDir: "/tmp/mock-data",
	}

	err := ensureStatePersistence(cfg)
	if err != nil {
		t.Fatalf("expected no error in mock mode, got %v", err)
	}
}

func TestEnsureStatePersistence_DoesNotExistYet(t *testing.T) {
	tempDir := t.TempDir()
	tempVarLib := filepath.Join(tempDir, "var-lib-thread")
	tempState := filepath.Join(tempDir, "data")

	// Point target path to our temp directory for isolated testing
	origPath := varLibThreadPath
	varLibThreadPath = tempVarLib
	t.Cleanup(func() { varLibThreadPath = origPath })

	cfg := &config.Config{
		Runtime:  config.RuntimeModeHardware,
		StateDir: tempState,
	}

	err := ensureStatePersistence(cfg)
	if err != nil {
		t.Fatalf("expected successful redirect, got %v", err)
	}

	// Verify the symlink was created successfully
	info, err := os.Lstat(tempVarLib)
	if err != nil {
		t.Fatalf("failed to stat link: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected target to be a symlink")
	}

	// Verify target points to the correct persistent folder
	target, err := os.Readlink(tempVarLib)
	if err != nil {
		t.Fatalf("failed to read link target: %v", err)
	}

	expectedTarget := filepath.Join(tempState, "otbr")
	if target != expectedTarget {
		t.Fatalf("expected link target %q, got %q", expectedTarget, target)
	}
}

func TestEnsureStatePersistence_AlreadySymlink(t *testing.T) {
	tempDir := t.TempDir()
	tempVarLib := filepath.Join(tempDir, "var-lib-thread")
	tempState := filepath.Join(tempDir, "data")

	origPath := varLibThreadPath
	varLibThreadPath = tempVarLib
	t.Cleanup(func() { varLibThreadPath = origPath })

	cfg := &config.Config{
		Runtime:  config.RuntimeModeHardware,
		StateDir: tempState,
	}

	// First run to establish the symlink
	if err := ensureStatePersistence(cfg); err != nil {
		t.Fatalf("failed first run setup: %v", err)
	}

	// Second run should return immediately without error
	err := ensureStatePersistence(cfg)
	if err != nil {
		t.Fatalf("expected no error on subsequent runs when symlink exists, got %v", err)
	}
}

func TestEnsureStatePersistence_MigrateNativeDirectory(t *testing.T) {
	tempDir := t.TempDir()
	tempVarLib := filepath.Join(tempDir, "var-lib-thread")
	tempState := filepath.Join(tempDir, "data")

	origPath := varLibThreadPath
	varLibThreadPath = tempVarLib
	t.Cleanup(func() { varLibThreadPath = origPath })

	// Create a native directory and seed files in it before ensuring persistence
	if err := os.MkdirAll(tempVarLib, 0o750); err != nil {
		t.Fatalf("failed to create fake native dir: %v", err)
	}

	fakeFile := filepath.Join(tempVarLib, "thread-dataset-active")
	fakeContent := []byte("active-dataset-key-data")
	if err := os.WriteFile(fakeFile, fakeContent, 0o600); err != nil {
		t.Fatalf("failed to seed native file: %v", err)
	}

	cfg := &config.Config{
		Runtime:  config.RuntimeModeHardware,
		StateDir: tempState,
	}

	// Run redirection with state migration
	if err := ensureStatePersistence(cfg); err != nil {
		t.Fatalf("expected successful migration and redirection, got %v", err)
	}

	// Verify file was migrated to /data/otbr/
	migratedFile := filepath.Join(tempState, "otbr", "thread-dataset-active")
	content, err := os.ReadFile(migratedFile) //nolint:gosec // G304: path confined to test temp StateDir/otbr
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	if string(content) != string(fakeContent) {
		t.Fatalf("expected content %q, got %q", string(fakeContent), string(content))
	}

	// Verify original native directory has become a symlink
	info, err := os.Lstat(tempVarLib)
	if err != nil {
		t.Fatalf("failed to lstat link: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("expected target to be a symlink")
	}
}
