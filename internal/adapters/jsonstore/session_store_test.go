package jsonstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/session"
)

func TestAppendRejectsChainBreakingReceipt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := SessionStore{Root: root}
	// A receipt entry whose ledger_head does not chain from the current head must be
	// rejected and must not be persisted, so a stale or concurrently minted receipt
	// is never reported as anchored on a valid chain.
	entry := session.Entry{
		Type:          session.EntryReceipt,
		Status:        "pass",
		ReceiptDigest: "digest",
		LedgerHead:    "does-not-chain",
	}
	if _, err := store.Append(context.Background(), "task", entry, "now"); err == nil {
		t.Fatal("append must fail closed on a chain-breaking receipt entry")
	}
	if _, err := store.Load(context.Background(), "task"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("a rejected append must not persist the session: err=%v", err)
	}
}

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

func TestAppendFallsBackWhenCachedLedgerHeadMismatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := SessionStore{Root: root}
	genesis := session.LedgerGenesisHead()
	receiptHead := session.NextLedgerHead(genesis, "digest")
	ledger := session.New("task", "now").WithEntry(session.Entry{
		Type:          session.EntryReceipt,
		RecordedAt:    "now",
		ReceiptDigest: "digest",
		LedgerHead:    receiptHead,
	})
	ledger.LedgerHead = "stale-cache"
	writeRawSession(t, root, ledger)

	got, err := store.Append(context.Background(), "task", session.Entry{Type: "criterion", CriterionID: "ac", Status: "pass"}, "later")
	if err != nil {
		t.Fatal(err)
	}
	if !got.LedgerValid || got.LedgerHead != receiptHead {
		t.Fatalf("append should fall back to full replay and repair ledger head: %+v", got)
	}
	if len(got.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(got.Entries))
	}
}

func TestListUsesMetadataReplayWithoutReceiptDecode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := SessionStore{Root: root}
	genesis := session.LedgerGenesisHead()
	ledger := session.New("task", "now").WithEntry(session.Entry{
		Type:          session.EntryReceipt,
		RecordedAt:    "now",
		ReceiptDigest: "digest",
		LedgerHead:    session.NextLedgerHead(genesis, "digest"),
		Output:        "{not valid receipt json",
	})
	writeRawSession(t, root, ledger)

	listed, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || !listed[0].LedgerValid {
		t.Fatalf("metadata list should not decode receipt bodies: %+v", listed)
	}
	loaded, err := store.Load(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LedgerValid {
		t.Fatalf("full load should still reject invalid receipt output: %+v", loaded)
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

func TestSessionStoreMultiProcessAppendContention(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const writers = 8
	cmds := make([]*exec.Cmd, 0, writers)
	outputs := make([]*bytes.Buffer, 0, writers)
	for i := 0; i < writers; i++ {
		cmd := exec.Command(os.Args[0], "-test.run", "^TestSessionStoreAppendHelperProcess$")
		cmd.Env = append(os.Environ(),
			"SCAFLD_JSONSTORE_APPEND_HELPER=1",
			"SCAFLD_JSONSTORE_ROOT="+root,
			"SCAFLD_JSONSTORE_ENTRY="+strconv.Itoa(i),
		)
		out := &bytes.Buffer{}
		cmd.Stdout = out
		cmd.Stderr = out
		outputs = append(outputs, out)
		if err := cmd.Start(); err != nil {
			t.Fatalf("start helper %d: %v", i, err)
		}
		cmds = append(cmds, cmd)
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatalf("helper %d failed: %v\n%s", i, err, outputs[i].String())
		}
	}
	ledger, err := (SessionStore{Root: root}).Load(context.Background(), "task")
	if err != nil {
		t.Fatal(err)
	}
	if !ledger.LedgerValid {
		t.Fatalf("ledger invalid: %s", ledger.LedgerError)
	}
	if len(ledger.Entries) != writers {
		t.Fatalf("entries = %d, want %d: %+v", len(ledger.Entries), writers, ledger.Entries)
	}
}

func TestSessionStoreAppendHelperProcess(t *testing.T) {
	if os.Getenv("SCAFLD_JSONSTORE_APPEND_HELPER") != "1" {
		t.Skip("helper process")
	}
	idx := os.Getenv("SCAFLD_JSONSTORE_ENTRY")
	root := os.Getenv("SCAFLD_JSONSTORE_ROOT")
	if root == "" || idx == "" {
		t.Fatal("helper environment missing root or entry")
	}
	_, err := (SessionStore{Root: root}).Append(context.Background(), "task", session.Entry{
		Type:        "criterion",
		CriterionID: "ac-" + idx,
		Status:      "pass",
		Output:      "helper " + idx,
	}, "now-"+idx)
	if err != nil {
		t.Fatal(err)
	}
}

func writeRawSession(t *testing.T, root string, ledger session.Session) {
	t.Helper()
	path := filepath.Join(root, ".scafld", "runs", ledger.TaskID, "session.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
