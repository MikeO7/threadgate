package hassdev

import (
	"context"
	"fmt"
)

// EnsureCoreConfig sets default location/country (USA) so HA does not show repairs.
func EnsureCoreConfig(ctx context.Context, cfg Config, accessToken string) error {
	if accessToken == "" {
		return fmt.Errorf("no access token")
	}
	c, err := dialWS(ctx, cfg.HAURL, accessToken)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	_, err = c.call(ctx, "config/core/update", map[string]any{
		"country":     cfg.HACountry,
		"currency":    cfg.HACurrency,
		"unit_system": cfg.HAUnitSystem,
		"time_zone":   cfg.HATimezone,
		"latitude":    cfg.HALatitude,
		"longitude":   cfg.HALongitude,
		"elevation":   0,
	})
	return err
}
