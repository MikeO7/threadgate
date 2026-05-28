package radio

import "github.com/MikeO7/threadgate/src/manager/internal/config"

// Resolver resolves spinel URLs and serial profiles from orchestrator config.
type Resolver struct {
	cfg Config
}

// NewResolver creates a radio resolver from full orchestrator config.
func NewResolver(cfg *config.Config) *Resolver {
	return &Resolver{cfg: ConfigFrom(cfg)}
}

// Resolve returns the connection profile, optionally forcing USB re-discovery.
func (r *Resolver) Resolve(forceDiscover bool) (Profile, error) {
	return ResolveProfile(r.cfg, forceDiscover)
}
