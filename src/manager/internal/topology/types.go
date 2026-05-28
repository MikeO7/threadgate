package topology

const (
	keyExtAddr     = "ExtAddr"
	keyRloc16      = "Rloc16"
	keyLinkQuality = "LinkQuality"

	linkKindDirect = "direct"
	linkKindChild  = "child"
	linkKindRoute  = "route"
)

// CollectMode controls how ot-ctl collection failures are handled during snapshot builds.
type CollectMode int

const (
	// CollectBestEffort returns partial snapshots with warnings when commands fail.
	CollectBestEffort CollectMode = iota
	// CollectStrict fails the entire snapshot on the first ot-ctl error.
	CollectStrict
)

// BuildOptions configures snapshot collection from ot-ctl.
type BuildOptions struct {
	Mode CollectMode
}

// Snapshot is the unified topology model for API JSON and dashboard rendering.
type Snapshot struct {
	State         string              `json:"state"`
	NetworkName   string              `json:"networkName"`
	ExtAddress    string              `json:"extAddress"`
	PanID         string              `json:"panId"`
	Channel       string              `json:"channel"`
	Rloc16        string              `json:"rloc16"`
	Neighbors     []Neighbor          `json:"neighbors"`
	MeshLinks     []MeshLink          `json:"meshLinks"`
	RoutingTree   RoutingTree         `json:"routingTree"`
	TrafficPaths  map[string][]string `json:"trafficPaths"`
	IPAddresses   []string            `json:"ipAddresses"`
	Counters      []Counter           `json:"counters"`
	ActiveDataset string              `json:"activeDataset,omitempty"`
	Warnings      []string            `json:"warnings,omitempty"`
}

type Neighbor struct {
	ExtAddr     string `json:"extAddr"`
	Rloc16      string `json:"rloc16"`
	LQI         int    `json:"lqi"`
	PathCost    int    `json:"pathCost"`
	NextHopRloc string `json:"nextHopRloc,omitempty"`
	Role        string `json:"role,omitempty"`
}

type ChildEntry struct {
	ID      int
	Rloc16  string
	ExtAddr string
	LQI     int
}

type RouterEntry struct {
	ID          int
	Rloc16      string
	NextHopID   int
	PathCost    int
	ExtAddr     string
	LinkQuality int
}

type Counter struct {
	Key string
	Val string
}

type MeshLink struct {
	FromRloc16 string `json:"from"`
	ToRloc16   string `json:"to"`
	Kind       string `json:"kind"`
	PathCost   int    `json:"pathCost"`
}

type RoutingParentEntry struct {
	Parent string   `json:"parent"`
	Link   MeshLink `json:"link"`
}

type RoutingTree struct {
	ParentOf   map[string]RoutingParentEntry `json:"parentOf"`
	ChildrenOf map[string][]string           `json:"childrenOf"`
	GatewayKey string                        `json:"gatewayKey"`
}

