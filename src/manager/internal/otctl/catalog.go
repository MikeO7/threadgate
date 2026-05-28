package otctl

import "strings"

// Command is one ot-ctl invocation: snapshot label (when used in topology) and CLI args.
type Command struct {
	Label string
	Args  []string
}

// Key returns the canonical mock and fixture lookup string (space-joined args).
func (c Command) Key() string {
	return strings.Join(c.Args, " ")
}

// Snapshot field labels (topology map keys).
const (
	LabelState         = "state"
	LabelNetworkName   = "networkname"
	LabelExtAddr       = "extaddr"
	LabelPanID         = "panid"
	LabelChannel       = "channel"
	LabelRloc16        = "rloc16"
	LabelDataset       = "dataset"
	LabelIPAddr        = "ipaddr"
	LabelNeighborTable = "neighbor table"
	LabelChildTable    = "child table"
	LabelRouterTable   = "router table"
	LabelCounters      = "counters"
)

var (
	State       = Command{Label: LabelState, Args: []string{"state"}}
	NetworkName = Command{Label: LabelNetworkName, Args: []string{"networkname"}}
	ExtAddr     = Command{Label: LabelExtAddr, Args: []string{"extaddr"}}
	PanID       = Command{Label: LabelPanID, Args: []string{"panid"}}
	Channel     = Command{Label: LabelChannel, Args: []string{"channel"}}
	Rloc16      = Command{Label: LabelRloc16, Args: []string{"rloc16"}}
	Counters    = Command{Label: LabelCounters, Args: []string{"counters"}}
	IPAddr      = Command{Label: LabelIPAddr, Args: []string{"ipaddr"}}

	NeighborTable = Command{Label: LabelNeighborTable, Args: []string{"neighbor", "table"}}
	ChildTable    = Command{Label: LabelChildTable, Args: []string{"child", "table"}}
	RouterTable   = Command{Label: LabelRouterTable, Args: []string{"router", "table"}}

	DatasetActive         = Command{Label: LabelDataset, Args: []string{"dataset", "active", "-x"}}
	DatasetPending        = Command{Args: []string{"dataset", "pending", "-x"}}
	DatasetSetActive      = Command{Args: []string{"dataset", "set", "active"}}
	DatasetCommitActive   = Command{Args: []string{"dataset", "commit", "active"}}
	DatasetSetPending     = Command{Args: []string{"dataset", "set", "pending"}}
	DatasetCommitPending  = Command{Args: []string{"dataset", "commit", "pending"}}
)

// SnapshotCommands is the full parallel collection used by topology.Build.
var SnapshotCommands = []Command{
	State,
	NetworkName,
	ExtAddr,
	PanID,
	Channel,
	Rloc16,
	DatasetActive,
	IPAddr,
	NeighborTable,
	ChildTable,
	RouterTable,
	Counters,
}
