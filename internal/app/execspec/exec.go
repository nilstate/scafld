package execspec

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/app/build"
)

type Input = build.Input
type Output = build.Output
type SpecStore = build.SpecStore
type SessionStore = build.SessionStore
type Runner = build.Runner
type Clock = build.Clock

func Run(ctx context.Context, specs SpecStore, sessions SessionStore, runner Runner, clock Clock, input Input) (Output, error) {
	return build.Run(ctx, specs, sessions, runner, clock, input)
}
