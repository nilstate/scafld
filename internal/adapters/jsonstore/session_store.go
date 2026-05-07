package jsonstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

// ErrSessionNotFound is returned when a task has no session ledger.
var ErrSessionNotFound = errors.New("session not found")

// SessionStore persists session ledgers below .scafld/runs.
type SessionStore struct {
	Root string
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
	return session.Replay(ledger), nil
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
		ledger, err := s.Load(ctx, taskID)
		if err != nil {
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
	mutex.Lock()
	defer mutex.Unlock()
	return s.writeLocked(path, session.Replay(ledger))
}

// Append appends one evidence entry and atomically writes the replayed ledger.
func (s SessionStore) Append(ctx context.Context, taskID string, entry session.Entry, now string) (session.Session, error) {
	if err := ctx.Err(); err != nil {
		return session.Session{}, err
	}
	path := s.path(taskID)
	mutex := pathLock(path)
	mutex.Lock()
	defer mutex.Unlock()
	ledger, err := s.loadUnlocked(path)
	if err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			return session.Session{}, err
		}
		ledger = session.New(taskID, now)
	}
	if entry.RecordedAt == "" {
		entry.RecordedAt = now
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("entry-%d", len(ledger.Entries)+1)
	}
	ledger = ledger.WithEntry(entry)
	if now != "" {
		ledger.UpdatedAt = now
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
	return session.Replay(ledger), nil
}

func (s SessionStore) writeLocked(path string, ledger session.Session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	data, err := json.MarshalIndent(session.Replay(ledger), "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	data = append(data, '\n')
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

func pathLock(path string) *sync.Mutex {
	value, _ := lockMap.LoadOrStore(path, &sync.Mutex{})
	return value.(*sync.Mutex)
}
