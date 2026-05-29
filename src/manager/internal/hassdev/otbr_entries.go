package hassdev

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// errOTBRNotFound means no Open Thread Border Router config entry exists in Home Assistant.
var errOTBRNotFound = errors.New("no otbr config entry")

// HAConfigEntry is a subset of Home Assistant config entry fields used by hassdev.
type HAConfigEntry struct {
	EntryID string `json:"entry_id"`
	Domain  string `json:"domain"`
	Title   string `json:"title"`
	State   string `json:"state"`
}

func listConfigEntries(ctx context.Context, cfg Config, token string) ([]HAConfigEntry, error) {
	http := newHTTPClient(cfg)
	data, err := http.get(ctx, "/api/config/config_entries/entry", token)
	if err != nil {
		return nil, err
	}
	var entries []HAConfigEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func findOTBREntry(ctx context.Context, cfg Config, token string) (*HAConfigEntry, error) {
	entries, err := listConfigEntries(ctx, cfg, token)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		if entries[i].Domain == haDomainOTBR {
			return &entries[i], nil
		}
	}
	return nil, errOTBRNotFound
}

func reloadOTBREntry(ctx context.Context, cfg Config, token, entryID string) error {
	http := newHTTPClient(cfg)
	_, err := http.postJSON(ctx, "/api/config/config_entries/entry/"+entryID+"/reload", nil, token)
	return err
}

func deleteOTBREntry(ctx context.Context, cfg Config, token, entryID string) error {
	http := newHTTPClient(cfg)
	_, err := http.delete(ctx, "/api/config/config_entries/entry/"+entryID, token)
	return err
}

// RepairOTBRIntegration reloads a failed OTBR entry or recreates it when reload is not enough.
func RepairOTBRIntegration(ctx context.Context, cfg Config, token string) error {
	entry, err := findOTBREntry(ctx, cfg, token)
	if err != nil {
		if errors.Is(err, errOTBRNotFound) {
			return EnsureOTBRIntegration(ctx, cfg, token)
		}
		return err
	}
	switch entry.State {
	case haEntryStateLoaded:
		_, _ = fmt.Fprintf(os.Stdout, "==> OTBR integration already loaded (%s)\n", entry.EntryID)
		return nil
	case "setup_error", "setup_retry", "failed_unload":
		_, _ = fmt.Fprintf(os.Stdout, "==> OTBR entry %s in state %q — reloading\n", entry.EntryID, entry.State)
		if err := reloadOTBREntry(ctx, cfg, token, entry.EntryID); err != nil {
			return fmt.Errorf("reload OTBR entry: %w", err)
		}
		if err := waitOTBREntryState(ctx, cfg, token, entry.EntryID, haEntryStateLoaded, 30*time.Second); err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "==> Reload did not reach loaded (%v) — removing and re-adding OTBR\n", err)
			if err := deleteOTBREntry(ctx, cfg, token, entry.EntryID); err != nil {
				return fmt.Errorf("delete OTBR entry: %w", err)
			}
			return EnsureOTBRIntegration(ctx, cfg, token)
		}
		_, _ = fmt.Fprintln(os.Stdout, "==> OTBR integration reloaded successfully")
		return nil
	default:
		_, _ = fmt.Fprintf(os.Stdout, "==> OTBR entry %s state=%q (expected loaded after setup)\n", entry.EntryID, entry.State)
		return nil
	}
}

func waitOTBREntryState(ctx context.Context, cfg Config, token, entryID, want string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entry, err := findOTBREntry(ctx, cfg, token)
		if err != nil {
			return err
		}
		if entry != nil && entry.EntryID == entryID && entry.State == want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for entry %s state %q", entryID, want)
}
