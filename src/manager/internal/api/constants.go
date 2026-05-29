package api

import "github.com/MikeO7/threadgate/src/manager/internal/thread"

var (
	activeDatasetHex  = thread.ValidOperationalDatasetHex
	pendingDatasetHex = thread.MockPendingDataset
)

const (
	mockNetworkName = "ThreadGate-Mock"

	jsonKeyStatus  = "status"
	jsonKeyMessage = "message"
	jsonStatusOK   = "ok"

	testNetworkName = "Thread-Test"
	testGatewayKey  = "c000"
	threadStateLeader = "leader"
)
