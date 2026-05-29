package thread

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

// ChannelScanResult holds the parsed and analyzed details for one 802.15.4 channel.
type ChannelScanResult struct {
	Channel        int    `json:"channel"`
	RSSI           int    `json:"rssi"`
	Rating         string `json:"rating"`
	Recommendation string `json:"recommendation"`
}
