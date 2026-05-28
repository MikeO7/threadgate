package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlightRecorder(t *testing.T) {
	StartFlightRecorder()
	if GlobalFlightRecorder == nil {
		t.Fatal("expected GlobalFlightRecorder to be initialized")
	}
	defer GlobalFlightRecorder.Stop()

	// Flush to a temporary file
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "test.trace")

	err := GlobalFlightRecorder.Flush(tracePath)
	if err != nil {
		t.Fatalf("failed to flush flight recorder: %v", err)
	}

	// Verify that the file exists and is not empty
	info, err := os.Stat(tracePath)
	if err != nil {
		t.Fatalf("expected trace file to exist: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected trace file to contain trace bytes, got size 0")
	}
}
