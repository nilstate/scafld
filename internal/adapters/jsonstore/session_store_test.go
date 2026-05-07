package jsonstore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/session"
)

func TestAtomicReplaceCleanup(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := SessionStore{Root: root}
	ledger := session.New("task", "now")
	if err := store.Save(context.Background(), ledger); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".scafld", "runs", "task", "session.json")); err != nil {
		t.Fatal(err)
	}
}

func TestListSessionsSorted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := SessionStore{Root: root}
	for _, taskID := range []string{"z-task", "a-task"} {
		if err := store.Save(context.Background(), session.New(taskID, "now")); err != nil {
			t.Fatal(err)
		}
	}
	ledgers, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ledgers) != 2 || ledgers[0].TaskID != "a-task" || ledgers[1].TaskID != "z-task" {
		t.Fatalf("ledgers = %+v", ledgers)
	}
}

func TestSessionWriteContentionRaceScenario(t *testing.T) {
	t.Parallel()

	store := SessionStore{Root: t.TempDir()}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.Append(context.Background(), "task", session.Entry{Type: "criterion", CriterionID: "ac", Status: "pass"}, "now")
			if err != nil {
				t.Errorf("append %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	ledger, err := store.Load(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Entries) != 8 {
		t.Fatalf("entries = %d, want 8", len(ledger.Entries))
	}
}
