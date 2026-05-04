package reconcile

import (
	"context"

	corereconcile "github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (spec.Model, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	model = corereconcile.FromSession(model, ledger)
	if err := specs.Save(ctx, path, model); err != nil {
		return spec.Model{}, err
	}
	return model, nil
}
