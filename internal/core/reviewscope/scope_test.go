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
