package update

import "context"

type Bundle interface {
	Refresh(context.Context) error
}

func Run(ctx context.Context, bundle Bundle) error {
	if bundle == nil {
		return nil
	}
	return bundle.Refresh(ctx)
}
