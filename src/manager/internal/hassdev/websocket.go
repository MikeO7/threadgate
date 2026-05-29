package hassdev

import (
	"context"

	"github.com/MikeO7/threadgate/src/manager/internal/hass"
)

type wsClient struct {
	inner *hass.WS
}

func dialWS(ctx context.Context, haURL, accessToken string) (*wsClient, error) {
	c, err := hass.Dial(ctx, haURL, accessToken)
	if err != nil {
		return nil, err
	}
	return &wsClient{inner: c}, nil
}

func (c *wsClient) Close() error {
	if c.inner == nil {
		return nil
	}
	return c.inner.Close()
}

func (c *wsClient) call(ctx context.Context, msgType string, fields map[string]any) (map[string]any, error) {
	return c.inner.Call(ctx, msgType, fields)
}

func createLongLivedToken(ctx context.Context, haURL, accessToken, clientName string, lifespan int) (string, error) {
	return hass.CreateLongLivedToken(ctx, haURL, accessToken, clientName, lifespan)
}
