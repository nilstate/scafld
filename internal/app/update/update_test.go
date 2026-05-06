package update

import (
	"context"
	"errors"
	"testing"
)

type fakeBundle struct {
	called bool
	err    error
}

func (f *fakeBundle) Refresh(context.Context) error {
	f.called = true
	return f.err
}

func TestRunNoopsWithoutBundle(t *testing.T) {
	t.Parallel()

	if err := Run(context.Background(), nil); err != nil {
		t.Fatalf("Run(nil) error = %v", err)
	}
}

func TestRunRefreshesBundle(t *testing.T) {
	t.Parallel()

	bundle := &fakeBundle{}
	if err := Run(context.Background(), bundle); err != nil {
		t.Fatal(err)
	}
	if !bundle.called {
		t.Fatal("bundle was not refreshed")
	}
}

func TestRunPropagatesRefreshError(t *testing.T) {
	t.Parallel()

	want := errors.New("refresh failed")
	err := Run(context.Background(), &fakeBundle{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
