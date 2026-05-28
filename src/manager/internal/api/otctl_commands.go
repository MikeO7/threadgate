package api

const (
	otctlCmdState                = "state"
	otctlCmdNetworkName          = "networkname"
	otctlCmdExtAddr              = "extaddr"
	otctlCmdPanID                = "panid"
	otctlCmdRloc16               = "rloc16"
	otctlCmdChannel              = "channel"
	otctlCmdCounters             = "counters"
	otctlCmdIPAddr               = "ipaddr"
	otctlCmdNeighborTable        = "neighbor table"
	otctlCmdChildTable           = "child table"
	otctlCmdRouterTable          = "router table"
	otctlArgTable                = "table"
	otctlCmdDatasetActiveX       = "dataset active -x"
	otctlCmdDatasetPendingX      = "dataset pending -x"
	otctlCmdDatasetCommitActive  = "dataset commit active"
	otctlCmdDatasetCommitPending = "dataset commit pending"

	activeDatasetHex  = "0e080000000000010000"
	pendingDatasetHex = "0e080000000000019999"
	mockNetworkName   = "ThreadGate-Mock"

	jsonKeyStatus = "status"
	jsonStatusOK  = "ok"

	testNetworkName = "Thread-Test"
	testGatewayKey  = "c000"
)
