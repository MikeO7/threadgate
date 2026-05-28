// Package runtime tracks orchestrator health and host audit state.
package runtime

import (
	"log"
	"os"
	"runtime/trace"
	"time"
)

// FlightRecorder wraps the Go runtime execution trace flight recorder.
type FlightRecorder struct {
	fr *trace.FlightRecorder
}

var (
	// GlobalFlightRecorder is the active flight recorder instance.
	GlobalFlightRecorder *FlightRecorder
)

// StartFlightRecorder starts the continuous tracing flight recorder.
func StartFlightRecorder() {
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
		MinAge:   5 * time.Second,
		MaxBytes: 10 * 1024 * 1024, // 10 MiB buffer
	})
	if err := fr.Start(); err != nil {
		log.Printf("[FlightRecorder] Failed to start: %v\n", err)
		return
	}
	GlobalFlightRecorder = &FlightRecorder{fr: fr}
	log.Println("[FlightRecorder] Continuous production execution tracing is active (low-overhead ring buffer).")
}

// Stop stops the flight recorder.
func (f *FlightRecorder) Stop() {
	if f != nil && f.fr != nil {
		f.fr.Stop()
	}
}

// Flush writes the current trace buffer to the specified file path.
func (f *FlightRecorder) Flush(path string) error {
	if f == nil || f.fr == nil {
		return os.ErrNotExist
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := f.fr.WriteTo(file); err != nil {
		return err
	}
	log.Printf("[FlightRecorder] Flushed active execution trace to: %s\n", path)
	return nil
}
