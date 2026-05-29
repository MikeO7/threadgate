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
	stateDisabled      = "disabled"
	stateOffline       = "offline"
	MockRouterCount    = 32 // Thread max active routers per partition
	MockDirectCount    = 8
	MockEndDeviceCount = 12
	MockMeshNodeCount  = MockRouterCount + MockEndDeviceCount
	MockNetworkName = "ThreadGate-Mock"
)

// Mock datasets use a non-default network key so Home Assistant does not raise insecure_thread_network.
var (
	MockActiveDataset  = ValidOperationalDatasetHex
	MockPendingDataset = mustBuildOperationalDatasetHex(MockNetworkKeyHex, 2)
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
	for i := range count {
		if i > 0 {
			b.WriteByte('\n')
		}
		id := i + 1
		extAddr := fmt.Sprintf("%016x", id)
		rloc16 := fmt.Sprintf("0xc%03x", id)
		lqi := (i % 3) + 1
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
	for i := range MockEndDeviceCount {
		if i > 0 {
			b.WriteByte('\n')
		}
		id := 100 + i
		extAddr := fmt.Sprintf("e00000000000%04x", i)
		rloc16 := fmt.Sprintf("0xc1%02x", i+0x40)
		lqi := (i % 3) + 1
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
	for i := range count {
		if i > 0 {
			b.WriteByte('\n')
		}
		id := i + 1
		extAddr := fmt.Sprintf("%016x", id)
		rloc16 := fmt.Sprintf("0xc%03x", id)
		nextHop, pathCost := mockRouteParent(id)
		lqi := (i % 3) + 1

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
	leaderdata    string
	prefix        string
	scanEnergy    string
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
		leaderdata:    "Partition ID: 2271874287\nWeighting: 64\nNetwork Data Version: 111\nStable Network Data Version: 112\nLeader Router ID: 50\nDone",
		prefix:        "fd11:22::/64 paros med stable\nDone",
		scanEnergy:    "| Ch | RSSI |\n+----+------+\n| 11 |  -82 |\n| 12 |  -85 |\n| 13 |  -89 |\n| 14 |  -87 |\n| 15 |  -92 |\n| 16 |  -86 |\n| 17 |  -86 |\n| 18 |  -72 |\n| 19 |  -65 |\n| 20 |  -90 |\n| 21 |  -84 |\n| 22 |  -82 |\n| 23 |  -80 |\n| 24 |  -78 |\n| 25 |  -95 |\n| 26 |  -98 |\nDone",
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
	otctl.State.Key():          func(s *mockStateData) string { return s.state },
	otctl.Rloc16.Key():         func(s *mockStateData) string { return s.rloc16 },
	otctl.ExtAddr.Key():        func(s *mockStateData) string { return s.extaddr },
	otctl.NetworkName.Key():    func(s *mockStateData) string { return s.networkname },
	otctl.PanID.Key():          func(s *mockStateData) string { return s.panid },
	otctl.Channel.Key():        func(s *mockStateData) string { return s.channel },
	otctl.Counters.Key():       func(s *mockStateData) string { return s.counters },
	otctl.IPAddr.Key():         func(s *mockStateData) string { return s.ipaddr },
	otctl.NeighborTable.Key():  func(s *mockStateData) string { return s.neighborTable },
	otctl.ChildTable.Key():     func(s *mockStateData) string { return s.childTable },
	otctl.RouterTable.Key():    func(s *mockStateData) string { return s.routerTable },
	otctl.DatasetActive.Key():  func(s *mockStateData) string { return s.activeHex },
	otctl.DatasetPending.Key(): func(s *mockStateData) string { return s.pendingHex },
	otctl.LeaderData.Key():     func(s *mockStateData) string { return s.leaderdata },
	otctl.PrefixTable.Key():    func(s *mockStateData) string { return s.prefix },
	otctl.ScanEnergy.Key():     func(s *mockStateData) string { return s.scanEnergy },
}

func runMockDatasetCommand(state *mockStateData, cmd string) (string, error) {
	if result, ok := runMockDatasetSetCommand(state, cmd); ok {
		return result, nil
	}
	if result, ok := runMockThreadLifecycleCommand(state, cmd); ok {
		return result, nil
	}
	return "", fmt.Errorf("mock ot-ctl: unknown command: %s", cmd)
}

func runMockDatasetSetCommand(state *mockStateData, cmd string) (string, bool) {
	switch {
	case strings.HasPrefix(cmd, "dataset set active "):
		state.activeHex = strings.TrimPrefix(cmd, "dataset set active ")
		return mockDone, true
	case cmd == otctl.DatasetCommitActive.Key():
		return mockDone, true
	case strings.HasPrefix(cmd, "dataset set pending "):
		state.pendingHex = strings.TrimPrefix(cmd, "dataset set pending ")
		return mockDone, true
	case cmd == otctl.DatasetCommitPending.Key():
		return mockDone, true
	default:
		return "", false
	}
}

func runMockThreadLifecycleCommand(state *mockStateData, cmd string) (string, bool) {
	switch cmd {
	case otctl.IfconfigUp.Key(), otctl.ThreadStart.Key():
		return mockActivateThread(state), true
	case otctl.IfconfigDown.Key():
		state.state = stateDisabled
		return mockDone, true
	case otctl.ThreadStop.Key():
		state.state = stateOffline
		return mockDone, true
	default:
		return "", false
	}
}

func mockActivateThread(state *mockStateData) string {
	if state.state == stateDisabled || state.state == stateOffline {
		state.state = mockLeader
	}
	return mockDone
}
