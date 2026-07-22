package jsonstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
	"github.com/nilstate/scafld/v2/internal/platform/filelock"
)

// ErrSessionNotFound is returned when a task has no session ledger.
var ErrSessionNotFound = errors.New("session not found")

// SessionStore persists session ledgers below .scafld/runs.
type SessionStore struct {
	Root         string
	TrustChecker session.ReceiptTrustChecker
}

var lockMap sync.Map

// Load reads and replays the session ledger for taskID.
func (s SessionStore) Load(ctx context.Context, taskID string) (session.Session, error) {
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	path := s.path(taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return session.Session{}, fmt.Errorf("%w: %s", ErrSessionNotFound, taskID)
		}
		return session.Session{}, fmt.Errorf("read session: %w", err)
	}
	var ledger session.Session
	if err := json.Unmarshal(data, &ledger); err != nil {
		return session.Session{}, fmt.Errorf("parse session: %w", err)
	}
	return s.replay(ledger), nil
}

// List reads all session ledgers under .scafld/runs.
func (s SessionStore) List(ctx context.Context) ([]session.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := s.Root
	if root == "" {
		root = "."
	}
	base := filepath.Join(root, ".scafld", "runs")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions: %w", err)
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	ledgers := make([]session.Session, 0, len(ids))
	for _, taskID := range ids {
		ledger, err := s.loadMetadata(ctx, taskID)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				continue
			}
			return nil, err
		}
		ledgers = append(ledgers, ledger)
	}
	return ledgers, nil
}

// Save atomically replaces the session ledger.
func (s SessionStore) Save(ctx context.Context, ledger session.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.path(ledger.TaskID)
	mutex := pathLock(path)
	releaseMutex, err := lockMutexContext(ctx, mutex)
	if err != nil {
		return err
	}
	defer releaseMutex()
	release, err := lockSessionFile(ctx, path)
	if err != nil {
		return err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.writeLocked(path, s.replay(ledger))
}

// Append appends one evidence entry and atomically writes the replayed ledger.
func (s SessionStore) Append(ctx context.Context, taskID string, entry session.Entry, now string) (session.Session, error) {
	return s.AppendTransaction(ctx, taskID, now, func(session.Session) ([]session.Entry, error) {
		return []session.Entry{entry}, nil
	})
}

// AppendTransaction derives and appends zero or more entries while holding the
// session file lock. Use it when the decision to append depends on the current
// ledger head and must not race another scafld process.
func (s SessionStore) AppendTransaction(ctx context.Context, taskID string, now string, derive func(session.Session) ([]session.Entry, error)) (session.Session, error) {
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	if derive == nil {
		return session.Session{}, errors.New("append transaction requires derive function")
	}
	path := s.path(taskID)
	mutex := pathLock(path)
	releaseMutex, err := lockMutexContext(ctx, mutex)
	if err != nil {
		return session.Session{}, err
	}
	defer releaseMutex()
	release, err := lockSessionFile(ctx, path)
	if err != nil {
		return session.Session{}, err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	ledger, err := s.loadRawUnlocked(path)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			return session.Session{}, err
		}
		ledger = session.New(taskID, now)
	} else {
		ledger = s.prepareForAppend(ledger)
	}
	entries, err := derive(ledger)
	if err != nil {
		return session.Session{}, err
	}
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	priorValid := ledger.LedgerValid
	for _, entry := range entries {
		if entry.RecordedAt == "" {
			entry.RecordedAt = now
		}
		if entry.ID == "" {
			entry.ID = fmt.Sprintf("entry-%d", len(ledger.Entries)+1)
		}
		ledger = session.AppendEntryWithOptions(ledger, entry, session.ReplayOptions{ReceiptTrustChecker: s.TrustChecker})
		// Fail closed without persisting when this entry breaks a previously valid
		// receipt chain (e.g. a stale or concurrently minted receipt whose ledger_head
		// does not chain from the current head). Leaving the entry out keeps the durable
		// ledger a valid chain and stops a mismatched receipt being reported as anchored.
		if priorValid && !ledger.LedgerValid {
			return session.Session{}, fmt.Errorf("append rejected: entry breaks ledger chain: %s", ledger.LedgerError)
		}
	}
	if now != "" {
		ledger.UpdatedAt = now
	}
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	if err := s.writeLocked(path, ledger); err != nil {
		return session.Session{}, err
	}
	return ledger, nil
}

func (s SessionStore) path(taskID string) string {
	root := s.Root
	if root == "" {
		root = "."
	}
	return filepath.Join(root, ".scafld", "runs", taskID, "session.json")
}

func (s SessionStore) loadUnlocked(path string) (session.Session, error) {
	ledger, err := s.loadRawUnlocked(path)
	if err != nil {
		return session.Session{}, err
	}
	return s.replay(ledger), nil
}

func (s SessionStore) loadRawUnlocked(path string) (session.Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return session.Session{}, ErrSessionNotFound
		}
		return session.Session{}, fmt.Errorf("read session: %w", err)
	}
	var ledger session.Session
	if err := json.Unmarshal(data, &ledger); err != nil {
		return session.Session{}, fmt.Errorf("parse session: %w", err)
	}
	return ledger, nil
}

func (s SessionStore) writeLocked(path string, ledger session.Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

func (s SessionStore) loadMetadata(ctx context.Context, taskID string) (session.Session, error) {
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	path := s.path(taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return session.Session{}, fmt.Errorf("%w: %s", ErrSessionNotFound, taskID)
		}
		return session.Session{}, fmt.Errorf("read session: %w", err)
	}
	var ledger session.Session
	if err := json.Unmarshal(data, &ledger); err != nil {
		return session.Session{}, fmt.Errorf("parse session: %w", err)
	}
	return session.ReplayWithOptions(ledger, session.ReplayOptions{SkipReceiptDecode: true}), nil
}

func (s SessionStore) replay(ledger session.Session) session.Session {
	return session.ReplayWithOptions(ledger, session.ReplayOptions{ReceiptTrustChecker: s.TrustChecker})
}

func (s SessionStore) prepareForAppend(ledger session.Session) session.Session {
	if canAppendFromCachedHead(ledger) {
		return ledger
	}
	return s.replay(ledger)
}

func canAppendFromCachedHead(ledger session.Session) bool {
	if !ledger.LedgerValid || ledger.LedgerHead == "" || ledger.CriterionStates == nil || ledger.PhaseBlocks == nil {
		return false
	}
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if strings.TrimSpace(entry.ReceiptDigest) == "" {
			continue
		}
		return strings.TrimSpace(entry.LedgerHead) == strings.TrimSpace(ledger.LedgerHead)
	}
	return strings.TrimSpace(ledger.LedgerHead) == session.LedgerGenesisHead()
}

func pathLock(path string) *sync.Mutex {
	value, _ := lockMap.LoadOrStore(path, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func lockMutexContext(ctx context.Context, mutex *sync.Mutex) (func(), error) {
	acquired := make(chan struct{})
	release := make(chan struct{})
	go func() {
		mutex.Lock()
		close(acquired)
		<-release
		mutex.Unlock()
	}()
	select {
	case <-acquired:
		return func() { close(release) }, nil
	case <-ctx.Done():
		go func() {
			<-acquired
			close(release)
		}()
		return nil, ctx.Err()
	}
}

// lockSessionFile fences concurrent scafld processes (e.g. two finalize calls
// on one task) so a read-modify-write cannot silently drop the other process's
// ledger entry. The in-process pathLock mutex must already be held.
func lockSessionFile(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	release, err := filelock.LockContext(ctx, path+".lock")
	if err != nil {
		return nil, fmt.Errorf("lock session: %w", err)
	}
	return release, nil
}
