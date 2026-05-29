package otbrapi

import (
	"context"

	"github.com/MikeO7/threadgate/src/manager/internal/thread"
)

// ThreadOps is the seam for OTBR-compatible Thread operations.
type ThreadOps interface {
	NodeInfo(ctx context.Context) (thread.NodeInfo, error)
	GetActiveDataset(ctx context.Context) (string, error)
	SetActiveDataset(ctx context.Context, hexStr string) error
	GetPendingDataset(ctx context.Context) (string, error)
	SetPendingDataset(ctx context.Context, hexStr string) error
	SetNodeState(ctx context.Context, enable bool) error
}

// ClientAdapter satisfies ThreadOps using a thread.Client.
type ClientAdapter struct {
	Client *thread.Client
}

func (a *ClientAdapter) NodeInfo(ctx context.Context) (thread.NodeInfo, error) {
	return a.Client.NodeInfo(ctx)
}

func (a *ClientAdapter) GetActiveDataset(ctx context.Context) (string, error) {
	return a.Client.GetActiveDataset(ctx)
}

func (a *ClientAdapter) SetActiveDataset(ctx context.Context, hexStr string) error {
	return a.Client.SetActiveDataset(ctx, hexStr)
}

func (a *ClientAdapter) GetPendingDataset(ctx context.Context) (string, error) {
	return a.Client.GetPendingDataset(ctx)
}

func (a *ClientAdapter) SetPendingDataset(ctx context.Context, hexStr string) error {
	return a.Client.SetPendingDataset(ctx, hexStr)
}

func (a *ClientAdapter) SetNodeState(ctx context.Context, enable bool) error {
	return a.Client.SetNodeState(ctx, enable)
}
