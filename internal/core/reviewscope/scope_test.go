package reviewscope

import (
	"reflect"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

func TestProjectClassifiesTaskChangesAndAmbientDrift(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Context: spec.Context{
			FilesImpacted: []string{"`api/handler.go`"},
		},
	}
	baseline := []string{
		" M old api/handler.go",
		" M old docs/index.md",
		" M old .scafld/runs/task/session.json",
	}
	current := []string{
		" M new api/handler.go",
		" M new docs/index.md",
		" M runtime .scafld/runs/task/session.json",
		"?? added api/new.go",
	}

	projection := Project(model, nil, baseline, current)
	if !reflect.DeepEqual(projection.Scope, []string{"api/handler.go"}) {
		t.Fatalf("scope = %+v", projection.Scope)
	}
	if got := coreworkspace.MutationStrings(projection.TaskChanges); !reflect.DeepEqual(got, []string{"changed api/handler.go (M old -> M new)"}) {
		t.Fatalf("task changes = %+v", got)
	}
	if got := coreworkspace.MutationStrings(projection.AmbientDrift); !reflect.DeepEqual(got, []string{"added api/new.go (?? added)", "changed docs/index.md (M old -> M new)"}) {
		t.Fatalf("ambient drift = %+v", got)
	}
}

func TestDeriveFiltersPrivateAndLocalPaths(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Context: spec.Context{
			FilesImpacted: []string{
				"`api/handler.go`",
				"`.env.local`",
				"`.priv/token`",
				"`.scafld/config.local.yaml`",
			},
		},
	}
	if got := Derive(model, nil, nil); !reflect.DeepEqual(got, []string{"api/handler.go"}) {
		t.Fatalf("scope = %+v", got)
	}
}

func TestDeriveIgnoresProseThatOnlyLooksPathish(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Scope: []string{
			"Reuse/refactor existing surface instead of adding a new package.",
			"Run mlx_lm.server --model locally for smoke testing.",
			"Keep the adapter boundary thin.",
		},
		Touchpoints: []string{
			"`internal/app/review/review.go` - review orchestration",
			"`docs/review.md`, `docs/lifecycle.md`: contract docs",
		},
	}
	want := []string{"docs/lifecycle.md", "docs/review.md", "internal/app/review/review.go"}
	if got := Derive(model, nil, nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("scope = %+v, want %+v", got, want)
	}
}

func TestProjectFallsBackToChangedSetWhenNoDeclaredScope(t *testing.T) {
	t.Parallel()

	projection := Project(spec.Model{TaskID: "task"}, nil, nil, []string{
		" M api api/handler.go",
		"?? docs docs/review.md",
		" M priv .priv/token",
	})
	wantScope := []string{"api/handler.go", "docs/review.md"}
	if !reflect.DeepEqual(projection.Scope, wantScope) {
		t.Fatalf("scope = %+v, want %+v", projection.Scope, wantScope)
	}
	if got := coreworkspace.MutationStrings(projection.TaskChanges); !reflect.DeepEqual(got, []string{"added api/handler.go (M api)", "added docs/review.md (?? docs)"}) {
		t.Fatalf("task changes = %+v", got)
	}
	if got := coreworkspace.MutationStrings(projection.AmbientDrift); !reflect.DeepEqual(got, []string{"added .priv/token (M priv)"}) {
		t.Fatalf("ambient drift = %+v, want private path kept outside fallback scope", got)
	}
}

func TestLiteralKeepsTopLevelExtensionlessPaths(t *testing.T) {
	t.Parallel()

	got := Literal([]string{"Makefile", "Dockerfile", "./LICENSE", ".priv/token", ".env.local"})
	want := []string{"Dockerfile", "LICENSE", "Makefile"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("literal scope = %+v, want %+v", got, want)
	}
}
