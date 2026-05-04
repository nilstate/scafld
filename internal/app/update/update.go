package update

import "context"

// Bundle is the managed core refresh port.
type Bundle interface {
	Refresh(context.Context) error
}

// Run refreshes managed core assets if a bundle is provided.
func Run(ctx context.Context, bundle Bundle) error {
	if bundle == nil {
		return nil
	}
	return bundle.Refresh(ctx)
}
