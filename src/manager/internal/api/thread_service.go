package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MikeO7/threadgate/src/manager/internal/api/topology"
	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
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

// CollectMode controls how ot-ctl collection failures are handled during snapshot builds.
type CollectMode = topology.CollectMode

const (
	CollectBestEffort = topology.CollectBestEffort
	CollectStrict     = topology.CollectStrict
)

// ThreadService orchestrates ot-ctl behind one interface: node info, datasets, diagnostics, and topology snapshots.
type ThreadService struct {
	otctl       otctl.Runner
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

	for _, pair := range []struct {
		cmd *otctl.Command
		set func(string)
	}{
		{&otctl.State, func(v string) { info.State = v }},
		{&otctl.Rloc16, func(v string) { info.Rloc16 = v }},
		{&otctl.ExtAddr, func(v string) { info.ExtAddress = v }},
		{&otctl.NetworkName, func(v string) { info.NetworkName = v }},
		{&otctl.PanID, func(v string) { info.PanID = v }},
	} {
		if v, err := s.run(ctx, pair.cmd.Args...); err != nil {
			errs = append(errs, err)
		} else {
			pair.set(v)
		}
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

	if v, err := s.run(ctx, otctl.Counters.Args...); err != nil {
		errs = append(errs, err)
	} else {
		diag.Counters = splitLines(v)
	}
	if v, err := s.run(ctx, otctl.IPAddr.Args...); err != nil {
		errs = append(errs, err)
	} else {
		diag.IPAddresses = splitLines(v)
	}
	if v, err := s.run(ctx, otctl.NeighborTable.Args...); err != nil {
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
	return s.run(ctx, otctl.DatasetActive.Args...)
}

func (s *ThreadService) SetActiveDataset(ctx context.Context, hexStr string) error {
	if _, err := s.run(ctx, append(otctl.DatasetSetActive.Args, hexStr)...); err != nil {
		return err
	}
	_, err := s.run(ctx, otctl.DatasetCommitActive.Args...)
	return err
}

func (s *ThreadService) GetPendingDataset(ctx context.Context) (string, error) {
	return s.run(ctx, otctl.DatasetPending.Args...)
}

func (s *ThreadService) SetPendingDataset(ctx context.Context, hexStr string) error {
	if _, err := s.run(ctx, append(otctl.DatasetSetPending.Args, hexStr)...); err != nil {
		return err
	}
	_, err := s.run(ctx, otctl.DatasetCommitPending.Args...)
	return err
}

// BuildSnapshot collects ot-ctl output and builds the dashboard topology model.
func (s *ThreadService) BuildSnapshot(ctx context.Context) (topology.Snapshot, error) {
	return topology.Build(ctx, s.otctl, topology.BuildOptions{Mode: s.collectMode})
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
