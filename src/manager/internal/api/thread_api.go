package api

import "context"

// ThreadAPI is the seam consumed by HTTP handlers for Thread node operations.
type ThreadAPI interface {
	NodeInfo(ctx context.Context) (NodeInfo, error)
	Diagnostics(ctx context.Context) (Diagnostics, error)
	BuildSnapshot(ctx context.Context) (Snapshot, error)
	GetActiveDataset(ctx context.Context) (string, error)
	SetActiveDataset(ctx context.Context, hexStr string) error
	GetPendingDataset(ctx context.Context) (string, error)
	SetPendingDataset(ctx context.Context, hexStr string) error
	ExportBackup(ctx context.Context) (ConfigBackup, error)
	ImportBackup(ctx context.Context, backup ConfigBackup) error
}
