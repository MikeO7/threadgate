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
