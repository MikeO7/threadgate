package topology

import "github.com/MikeO7/threadgate/src/manager/internal/otctl"

const (
	keyExtAddr     = "ExtAddr"
	keyRloc16      = "Rloc16"
	keyLinkQuality = "LinkQuality"

	linkKindDirect = "direct"
	linkKindChild  = "child"
	linkKindRoute  = "route"
)

// BuildOptions configures snapshot collection from ot-ctl.
type BuildOptions struct {
	Policy otctl.Policy
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
	DeviceNames   map[string]string   `json:"deviceNames,omitempty"`
	LeaderData    LeaderData          `json:"leaderData"`
	Prefixes      []PrefixEntry       `json:"prefixes"`
}

// LeaderData holds partition and leader details.
type LeaderData struct {
	PartitionID       uint32 `json:"partitionId"`
	Weighting         int    `json:"weighting"`
	NetworkDataVer    int    `json:"networkDataVersion"`
	StableNetworkData int    `json:"stableNetworkDataVersion"`
	LeaderRouterID    int    `json:"leaderRouterId"`
}

// PrefixEntry represents an active on-mesh IPv6 prefix configured in the Thread network.
type PrefixEntry struct {
	Prefix     string   `json:"prefix"`
	Flags      []string `json:"flags"`
	Stable     bool     `json:"stable"`
	Preference string   `json:"preference"`
}

// Neighbor represents an adjacent Thread device reachable by this node.
type Neighbor struct {
	ExtAddr      string `json:"extAddr"`
	Rloc16       string `json:"rloc16"`
	LQI          int    `json:"lqi"`
	PathCost     int    `json:"pathCost"`
	NextHopRloc  string `json:"nextHopRloc,omitempty"`
	Role         string `json:"role,omitempty"`
	FriendlyName string `json:"friendlyName,omitempty"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
	SwVersion    string `json:"swVersion,omitempty"`
	Battery      string `json:"battery,omitempty"`
	Availability string `json:"availability,omitempty"`
	HassDeviceID string `json:"hassDeviceId,omitempty"`
}

// ChildEntry represents a sleepy or end-device connected directly to this node.
type ChildEntry struct {
	ID      int
	Rloc16  string
	ExtAddr string
	LQI     int
}

// RouterEntry represents an active router participating in the mesh backbone.
type RouterEntry struct {
	ID          int
	Rloc16      string
	NextHopID   int
	PathCost    int
	ExtAddr     string
	LinkQuality int
}

// Counter represents a generic diagnostic metric (e.g., packet counts).
type Counter struct {
	Key string
	Val string
}

// MeshLink defines a directed connection between two nodes in the Thread topology.
type MeshLink struct {
	FromRloc16 string `json:"from"`
	ToRloc16   string `json:"to"`
	Kind       string `json:"kind"`
	PathCost   int    `json:"pathCost"`
}

// RoutingParentEntry encapsulates a child-to-parent mapping for path traversal.
type RoutingParentEntry struct {
	Parent string   `json:"parent"`
	Link   MeshLink `json:"link"`
}

// RoutingTree models the hierarchical structure of the mesh for visual rendering.
type RoutingTree struct {
	ParentOf   map[string]RoutingParentEntry `json:"parentOf"`
	ChildrenOf map[string][]string           `json:"childrenOf"`
	GatewayKey string                        `json:"gatewayKey"`
}
