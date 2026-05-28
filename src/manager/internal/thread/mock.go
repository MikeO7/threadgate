package thread

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/MikeO7/threadgate/src/manager/internal/otctl"
)

const (
	mockDone           = "Done"
	mockLeader         = "leader"
	MockRouterCount    = 32 // Thread max active routers per partition
	MockDirectCount    = 8
	MockEndDeviceCount = 12
	MockMeshNodeCount  = MockRouterCount + MockEndDeviceCount
	MockNetworkName    = "ThreadGate-Mock"
	MockActiveDataset  = "0e080000000000010000"
	MockPendingDataset = "0e080000000000019999"
)

func mockRouteParent(id int) (nextHopID int, pathCost int) {
	switch {
	case id <= MockDirectCount:
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
	for i := 0; i < MockEndDeviceCount; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		id := 100 + i
		extAddr := fmt.Sprintf("e00000000000%04x", i)
		rloc16 := fmt.Sprintf("0xc1%02x", i+0x40)
		lqi := (i%3) + 1
		fmt.Fprintf(
			&b,
			"ID:%d Rloc16:%s ExtAddr:%s LinkQuality:%d",
			id, rloc16, extAddr, lqi,
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
		state:         mockLeader,
		rloc16:        "0xc000",
		extaddr:       "1122334455667788",
		networkname:   MockNetworkName,
		panid:         "0x1234",
		channel:       "15",
		activeHex:     MockActiveDataset,
		pendingHex:    MockPendingDataset,
		counters:      "MacTxUnique=100\nMacRxUnique=200\nTxRetry=5",
		ipaddr:        "fd11:22::1\nfe80::1",
		neighborTable: buildMockNeighborTable(MockDirectCount),
		childTable:    buildMockChildTable(),
		routerTable:   buildMockRouterTable(MockRouterCount),
	}
}

// Mock simulates ot-ctl with instance-scoped state (safe for parallel tests).
type Mock struct {
	mu    sync.RWMutex
	state mockStateData
}

// NewMock returns a mock ot-ctl adapter with default simulated Thread data.
func NewMock() *Mock {
	return &Mock{state: defaultMockState()}
}

func (m *Mock) Run(_ context.Context, args ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return runMockCommand(&m.state, otctl.Command{Args: args}.Key())
}

func runMockCommand(state *mockStateData, cmd string) (string, error) {
	if getter, ok := mockStateGetters[cmd]; ok {
		return getter(state), nil
	}
	return runMockDatasetCommand(state, cmd)
}

var mockStateGetters = map[string]func(*mockStateData) string{
	otctl.State.Key():         func(s *mockStateData) string { return s.state },
	otctl.Rloc16.Key():        func(s *mockStateData) string { return s.rloc16 },
	otctl.ExtAddr.Key():       func(s *mockStateData) string { return s.extaddr },
	otctl.NetworkName.Key():   func(s *mockStateData) string { return s.networkname },
	otctl.PanID.Key():         func(s *mockStateData) string { return s.panid },
	otctl.Channel.Key():       func(s *mockStateData) string { return s.channel },
	otctl.Counters.Key():      func(s *mockStateData) string { return s.counters },
	otctl.IPAddr.Key():        func(s *mockStateData) string { return s.ipaddr },
	otctl.NeighborTable.Key(): func(s *mockStateData) string { return s.neighborTable },
	otctl.ChildTable.Key():    func(s *mockStateData) string { return s.childTable },
	otctl.RouterTable.Key():    func(s *mockStateData) string { return s.routerTable },
	otctl.DatasetActive.Key(): func(s *mockStateData) string { return s.activeHex },
	otctl.DatasetPending.Key(): func(s *mockStateData) string { return s.pendingHex },
}

func runMockDatasetCommand(state *mockStateData, cmd string) (string, error) {
	switch {
	case strings.HasPrefix(cmd, "dataset set active "):
		state.activeHex = strings.TrimPrefix(cmd, "dataset set active ")
		return mockDone, nil
	case cmd == otctl.DatasetCommitActive.Key():
		return mockDone, nil
	case strings.HasPrefix(cmd, "dataset set pending "):
		state.pendingHex = strings.TrimPrefix(cmd, "dataset set pending ")
		return mockDone, nil
	case cmd == otctl.DatasetCommitPending.Key():
		return mockDone, nil
	}

	return "", fmt.Errorf("mock ot-ctl: unknown command: %s", cmd)
}
