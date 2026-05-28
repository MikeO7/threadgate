package api

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

const (
	constDone       = "Done"
	constLeader     = "leader"
	mockNodeCount   = 30
	mockDirectCount = 8
)

func mockRouteParent(id int) (nextHopID int, pathCost int) {
	switch {
	case id <= mockDirectCount:
		return id, 0
	case id <= 16:
		return id - 8, 1
	case id <= 22:
		return 9 + (id-17)%6, 2
	default:
		return 17 + (id-23)%6, 3
	}
}

func buildMockNeighborTable(count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		id := i + 1
		extAddr := fmt.Sprintf("%016x", id)
		rloc16 := fmt.Sprintf("0xc%03x", id)
		lqi := (i%3) + 1
		fmt.Fprintf(
			&b,
			"Role:R ExtAddr:%s Rloc16:%s LinkQuality:%d",
			extAddr, rloc16, lqi,
		)
	}
	return b.String()
}

func buildMockChildTable() string {
	var b strings.Builder
	for i := 27; i <= mockNodeCount; i++ {
		if i > 27 {
			b.WriteByte('\n')
		}
		extAddr := fmt.Sprintf("%016x", i)
		rloc16 := fmt.Sprintf("0xc%03x", i)
		lqi := (i%3) + 1
		fmt.Fprintf(
			&b,
			"ID:%d Rloc16:%s ExtAddr:%s LinkQuality:%d",
			i, rloc16, extAddr, lqi,
		)
	}
	return b.String()
}

func buildMockRouterTable(count int) string {
	var b strings.Builder
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		id := i + 1
		extAddr := fmt.Sprintf("%016x", id)
		rloc16 := fmt.Sprintf("0xc%03x", id)
		nextHop, pathCost := mockRouteParent(id)
		lqi := (i%3) + 1

		fmt.Fprintf(
			&b,
			"ID:%d Rloc16:%s NextHop:%d PathCost:%d ExtAddr:%s LinkQuality:%d",
			id, rloc16, nextHop, pathCost, extAddr, lqi,
		)
	}
	return b.String()
}

type mockStateData struct {
	state         string
	rloc16        string
	extaddr       string
	networkname   string
	panid         string
	channel       string
	activeHex     string
	pendingHex    string
	counters      string
	ipaddr        string
	neighborTable string
	childTable    string
	routerTable   string
}

func defaultMockState() mockStateData {
	return mockStateData{
		state:         constLeader,
		rloc16:        "0xc000",
		extaddr:       "1122334455667788",
		networkname:   mockNetworkName,
		panid:         "0x1234",
		channel:       "15",
		activeHex:     activeDatasetHex,
		pendingHex:    pendingDatasetHex,
		counters:      "MacTxUnique=100\nMacRxUnique=200\nTxRetry=5",
		ipaddr:        "fd11:22::1\nfe80::1",
		neighborTable: buildMockNeighborTable(mockDirectCount),
		childTable:    buildMockChildTable(),
		routerTable:   buildMockRouterTable(mockNodeCount),
	}
}

// MockOtCtl simulates ot-ctl with instance-scoped state (safe for parallel tests).
type MockOtCtl struct {
	mu    sync.RWMutex
	state mockStateData
}

// NewMockOtCtl returns a mock ot-ctl adapter with default simulated Thread data.
func NewMockOtCtl() *MockOtCtl {
	return &MockOtCtl{state: defaultMockState()}
}

func (m *MockOtCtl) Run(_ context.Context, args ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return runMockCommand(&m.state, otctl.Command{Args: args}.Key())
}

func runMockCommand(state *mockStateData, cmd string) (string, error) {
	switch cmd {
	case otctl.State.Key():
		return state.state, nil
	case otctl.Rloc16.Key():
		return state.rloc16, nil
	case otctl.ExtAddr.Key():
		return state.extaddr, nil
	case otctl.NetworkName.Key():
		return state.networkname, nil
	case otctl.PanID.Key():
		return state.panid, nil
	case otctl.Channel.Key():
		return state.channel, nil
	case otctl.Counters.Key():
		return state.counters, nil
	case otctl.IPAddr.Key():
		return state.ipaddr, nil
	case otctl.NeighborTable.Key():
		return state.neighborTable, nil
	case otctl.ChildTable.Key():
		return state.childTable, nil
	case otctl.RouterTable.Key():
		return state.routerTable, nil
	case otctl.DatasetActive.Key():
		return state.activeHex, nil
	case otctl.DatasetPending.Key():
		return state.pendingHex, nil
	}
	return runMockDatasetCommand(state, cmd)
}

func runMockDatasetCommand(state *mockStateData, cmd string) (string, error) {
	switch {
	case strings.HasPrefix(cmd, "dataset set active "):
		state.activeHex = strings.TrimPrefix(cmd, "dataset set active ")
		return constDone, nil
	case cmd == otctl.DatasetCommitActive.Key():
		return constDone, nil
	case strings.HasPrefix(cmd, "dataset set pending "):
		state.pendingHex = strings.TrimPrefix(cmd, "dataset set pending ")
		return constDone, nil
	case cmd == otctl.DatasetCommitPending.Key():
		return constDone, nil
	}

	return "", fmt.Errorf("mock ot-ctl: unknown command: %s", cmd)
}
