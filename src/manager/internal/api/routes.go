package api

import "net/http"

func (s *Server) registerRoutes(mux *http.ServeMux) {
	routes := []struct {
		pattern string
		handler http.HandlerFunc
	}{
		{"/api/node/channels/scan", s.handleChannelScan},
		{"/api/node", s.handleNodeInfo},
		{"/api/actions", s.otbr.HandleAPIActions},
		{"/api/health", s.handleHealth},
		{"/node/ba-id", s.otbr.HandleBorderAgentID},
		{"/node/ext-address", s.otbr.HandleExtAddress},
		{"/node/coprocessor/version", s.otbr.HandleCoprocessorVersion},
		{"/node/state", s.otbr.HandleNodeState},
		{"/node/dataset/active", s.otbr.HandleActiveDataset},
		{"/api/node/dataset/active", s.otbr.HandleActiveDataset},
		{"/api/node/dataset/active/decode", s.handleActiveDatasetDecode},
		{"/node/dataset/pending", s.otbr.HandlePendingDataset},
		{"/api/node/dataset/pending", s.otbr.HandlePendingDataset},
		{"/api/node/dataset/pending/decode", s.handlePendingDatasetDecode},
		{"/api/node/dataset/decode", s.handleDatasetDecode},
		{"/api/diagnostics/healthcheck", s.handleHealthcheck},
		{"/api/diagnostics", s.handleDiagnostics},
		{"/api/topology", s.handleTopology},
		{"/api/backup/import", s.handleBackup},
		{"/api/backup/save", s.handleBackup},
		{"/api/backup/files/", s.handleBackup},
		{"/api/backup/files", s.handleBackup},
		{"/api/backup", s.handleBackup},
		{"/api/trace/flush", s.handleTraceFlush},
		{"/api/pair/initiate", s.handlePairInitiate},
		{"/api/pair/status", s.handlePairStatus},
		{"/api/pair/active", s.handlePairActive},
		{"/api/pair/approve", s.handlePairApprove},
		{"/api/pair/deny", s.handlePairDeny},
		{"/logo.svg", s.handleLogo},
		{"/favicon.ico", s.handleLogo},
		{"/", s.handleDashboard},
	}
	for _, route := range routes {
		mux.HandleFunc(route.pattern, route.handler)
	}
}
