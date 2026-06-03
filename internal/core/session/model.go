package session

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
)

const (
	// EntryWorkspaceBaseline records dirty workspace state before task execution begins.
	EntryWorkspaceBaseline = "workspace_baseline"
	// EntryReceipt records a signed gate receipt in the append-only ledger.
	EntryReceipt = "receipt"
	ledgerDomain = "scafld-ledger-v1"
)

// Session is the durable task evidence ledger plus replayed state indexes.
type Session struct {
	SchemaVersion   int                    `json:"schema_version"`
	TaskID          string                 `json:"task_id"`
	CreatedAt       string                 `json:"created_at,omitempty"`
	UpdatedAt       string                 `json:"updated_at,omitempty"`
	Entries         []Entry                `json:"entries"`
	CriterionStates map[string]StateRecord `json:"criterion_states,omitempty"`
	PhaseBlocks     map[string]StateRecord `json:"phase_blocks,omitempty"`
	LedgerHead      string                 `json:"ledger_head,omitempty"`
	LedgerValid     bool                   `json:"ledger_valid,omitempty"`
	LedgerError     string                 `json:"ledger_error,omitempty"`
}

// Entry is one append-only evidence event in a session ledger.
type Entry struct {
	ID            string `json:"id,omitempty"`
	Type          string `json:"type"`
	RecordedAt    string `json:"recorded_at"`
	CriterionID   string `json:"criterion_id,omitempty"`
	PhaseID       string `json:"phase_id,omitempty"`
	Status        string `json:"status,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Command       string `json:"command,omitempty"`
	ExitCode      int    `json:"exit_code,omitempty"`
	Output        string `json:"output,omitempty"`
	Path          string `json:"path,omitempty"`
	ReceiptDigest string `json:"receipt_digest,omitempty"`
	LedgerHead    string `json:"ledger_head,omitempty"`
}

// StateRecord is the replayed state for a criterion or phase.
type StateRecord struct {
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
}

// WithEntry returns a replayed copy of the session with entry appended.
func (s Session) WithEntry(entry Entry) Session {
	next := s
	next.Entries = append(append([]Entry(nil), s.Entries...), entry)
	next = Replay(next)
	return next
}

// New creates an empty session ledger for taskID.
func New(taskID string, now string) Session {
	return Session{
		SchemaVersion:   1,
		TaskID:          taskID,
		CreatedAt:       now,
		UpdatedAt:       now,
		Entries:         []Entry{},
		CriterionStates: map[string]StateRecord{},
		PhaseBlocks:     map[string]StateRecord{},
		LedgerHead:      LedgerGenesisHead(),
		LedgerValid:     true,
	}
}

// Replay rebuilds derived criterion and phase state from session entries.
func Replay(s Session) Session {
	next := s
	next.CriterionStates = map[string]StateRecord{}
	next.PhaseBlocks = map[string]StateRecord{}
	next.LedgerHead = LedgerGenesisHead()
	next.LedgerValid = true
	next.LedgerError = ""
	for idx, entry := range s.Entries {
		source := entry.ID
		if source == "" {
			source = entry.Type
		}
		record := StateRecord{
			Status:    entry.Status,
			Reason:    entry.Reason,
			UpdatedAt: entry.RecordedAt,
			SourceID:  source,
		}
		if record.Status == "" {
			record.Status = entry.Type
		}
		if entry.CriterionID != "" {
			next.CriterionStates[entry.CriterionID] = record
		}
		if entry.PhaseID != "" {
			next.PhaseBlocks[entry.PhaseID] = record
		}
		if idx == len(s.Entries)-1 && entry.RecordedAt != "" {
			next.UpdatedAt = entry.RecordedAt
		}
		next = replayLedgerHead(next, entry)
	}
	return next
}

// LedgerGenesisHead returns the starting head for sessions with no receipts.
func LedgerGenesisHead() string {
	sum := sha256.Sum256([]byte(ledgerDomain + " genesis"))
	return hex.EncodeToString(sum[:])
}

// NextLedgerHead derives the next receipt-chain head from the prior head and receipt digest.
func NextLedgerHead(priorLedgerHead string, receiptDigest string) string {
	prior := strings.TrimSpace(priorLedgerHead)
	if prior == "" {
		prior = LedgerGenesisHead()
	}
	sum := sha256.Sum256([]byte(ledgerDomain + "\n" + prior + "\n" + strings.TrimSpace(receiptDigest)))
	return hex.EncodeToString(sum[:])
}

func replayLedgerHead(s Session, entry Entry) Session {
	digest := strings.TrimSpace(entry.ReceiptDigest)
	storedHead := strings.TrimSpace(entry.LedgerHead)
	if entry.Type == EntryReceipt && strings.TrimSpace(entry.Output) != "" {
		envelope, err := receipt.DecodeEnvelope([]byte(entry.Output))
		if err != nil {
			s.LedgerValid = false
			s.LedgerError = "invalid receipt entry: " + err.Error()
			return s
		}
		computed, err := receipt.ReceiptDigest(envelope.Body)
		if err != nil {
			s.LedgerValid = false
			s.LedgerError = "invalid receipt digest: " + err.Error()
			return s
		}
		digest = computed
		if storedHead == "" {
			storedHead = envelope.Body.LedgerHead
		}
	}
	if digest == "" {
		return s
	}
	expected := NextLedgerHead(s.LedgerHead, digest)
	if storedHead != "" && storedHead != expected {
		s.LedgerValid = false
		s.LedgerError = "receipt ledger_head mismatch"
		return s
	}
	s.LedgerHead = expected
	return s
}

// OrderedCriterionIDs returns criterion state keys in deterministic order.
func OrderedCriterionIDs(s Session) []string {
	ids := make([]string, 0, len(s.CriterionStates))
	for id := range s.CriterionStates {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// FirstWorkspaceBaseline returns the first captured task workspace baseline.
func FirstWorkspaceBaseline(s Session) (Entry, bool) {
	for _, entry := range s.Entries {
		if entry.Type == EntryWorkspaceBaseline {
			return entry, true
		}
	}
	return Entry{}, false
}

// WorkspaceBaselineSnapshot decodes the raw changed-file snapshot from an entry.
func WorkspaceBaselineSnapshot(entry Entry) []string {
	var snapshot []string
	for _, line := range strings.Split(entry.Output, "\n") {
		if strings.TrimSpace(line) != "" {
			snapshot = append(snapshot, strings.TrimRight(line, "\r"))
		}
	}
	return snapshot
}
