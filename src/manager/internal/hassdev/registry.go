package hassdev

import (
	"context"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
)

func countHADevicesWS(ctx context.Context, cfg Config, token string) (int, error) {
	return hass.CountDevices(ctx, cfg.HAURL, token)
}
