package api

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// NodeInfo holds REST-facing Thread node identity fields.
type NodeInfo struct {
	State       string `json:"State"`
	Rloc16      string `json:"Rloc16"`
	ExtAddress  string `json:"ExtAddress"`
	NetworkName string `json:"NetworkName"`
	PanID       string `json:"PanId"`
}

// Diagnostics holds raw diagnostic lines from ot-ctl.
type Diagnostics struct {
	IPAddresses   []string `json:"IPAddresses"`
	Counters      []string `json:"Counters"`
	NeighborTable []string `json:"NeighborTable"`
	Timestamp     string   `json:"Timestamp"`
}

// ThreadService orchestrates ot-ctl calls behind one interface.
type ThreadService struct {
	otctl       OtCtl
	collectMode CollectMode
}

// NewThreadService wires a ThreadService with the given ot-ctl adapter.
func NewThreadService(otctl OtCtl, collectMode CollectMode) *ThreadService {
	if collectMode != CollectStrict {
		collectMode = CollectBestEffort
	}
	return &ThreadService{otctl: otctl, collectMode: collectMode}
}

func (s *ThreadService) run(ctx context.Context, args ...string) (string, error) {
	return s.otctl.Run(ctx, args...)
}

// NodeInfo fetches node identity fields from ot-ctl.
func (s *ThreadService) NodeInfo(ctx context.Context) (NodeInfo, error) {
	var info NodeInfo
	var errs []error

	if v, err := s.run(ctx, otctlCmdState); err != nil {
		errs = append(errs, err)
	} else {
		info.State = v
	}
	if v, err := s.run(ctx, "rloc16"); err != nil {
		errs = append(errs, err)
	} else {
		info.Rloc16 = v
	}
	if v, err := s.run(ctx, otctlCmdExtAddr); err != nil {
		errs = append(errs, err)
	} else {
		info.ExtAddress = v
	}
	if v, err := s.run(ctx, otctlCmdNetworkName); err != nil {
		errs = append(errs, err)
	} else {
		info.NetworkName = v
	}
	if v, err := s.run(ctx, otctlCmdPanID); err != nil {
		errs = append(errs, err)
	} else {
		info.PanID = v
	}

	if len(errs) > 0 {
		return info, fmt.Errorf("node info: %w", errs[0])
	}
	return info, nil
}

// Diagnostics fetches counters, addresses, and the raw neighbor table.
func (s *ThreadService) Diagnostics(ctx context.Context) (Diagnostics, error) {
	var diag Diagnostics
	var errs []error

	if v, err := s.run(ctx, "counters"); err != nil {
		errs = append(errs, err)
	} else {
		diag.Counters = splitLines(v)
	}
	if v, err := s.run(ctx, "ipaddr"); err != nil {
		errs = append(errs, err)
	} else {
		diag.IPAddresses = splitLines(v)
	}
	if v, err := s.run(ctx, "neighbor", "table"); err != nil {
		errs = append(errs, err)
	} else {
		diag.NeighborTable = splitLines(v)
	}

	diag.Timestamp = time.Now().Format(time.RFC3339)
	if len(errs) > 0 {
		return diag, fmt.Errorf("diagnostics: %w", errs[0])
	}
	return diag, nil
}

func (s *ThreadService) GetActiveDataset(ctx context.Context) (string, error) {
	return s.run(ctx, "dataset", "active", "-x")
}

func (s *ThreadService) SetActiveDataset(ctx context.Context, hexStr string) error {
	if _, err := s.run(ctx, "dataset", "set", "active", hexStr); err != nil {
		return err
	}
	_, err := s.run(ctx, "dataset", "commit", "active")
	return err
}

func (s *ThreadService) GetPendingDataset(ctx context.Context) (string, error) {
	return s.run(ctx, "dataset", "pending", "-x")
}

func (s *ThreadService) SetPendingDataset(ctx context.Context, hexStr string) error {
	if _, err := s.run(ctx, "dataset", "set", "pending", hexStr); err != nil {
		return err
	}
	_, err := s.run(ctx, "dataset", "commit", "pending")
	return err
}

// BuildSnapshot collects ot-ctl output and builds the dashboard topology model.
func (s *ThreadService) BuildSnapshot(ctx context.Context) (Snapshot, error) {
	return BuildSnapshot(ctx, s.otctl, SnapshotBuildOptions{Mode: s.collectMode})
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
