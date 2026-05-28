package otctl

import "context"

// Runner executes ot-ctl subcommands. Production and mock adapters satisfy this seam.
type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
}
