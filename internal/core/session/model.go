package session

import "sort"

// Session is the durable task evidence ledger plus replayed state indexes.
type Session struct {
	SchemaVersion   int                    `json:"schema_version"`
	TaskID          string                 `json:"task_id"`
	CreatedAt       string                 `json:"created_at,omitempty"`
	UpdatedAt       string                 `json:"updated_at,omitempty"`
	Entries         []Entry                `json:"entries"`
	CriterionStates map[string]StateRecord `json:"criterion_states,omitempty"`
	PhaseBlocks     map[string]StateRecord `json:"phase_blocks,omitempty"`
}

// Entry is one append-only evidence event in a session ledger.
type Entry struct {
	ID          string `json:"id,omitempty"`
	Type        string `json:"type"`
	RecordedAt  string `json:"recorded_at"`
	CriterionID string `json:"criterion_id,omitempty"`
	PhaseID     string `json:"phase_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Command     string `json:"command,omitempty"`
	ExitCode    int    `json:"exit_code,omitempty"`
	Output      string `json:"output,omitempty"`
	Path        string `json:"path,omitempty"`
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
	}
}

// Replay rebuilds derived criterion and phase state from session entries.
func Replay(s Session) Session {
	next := s
	next.CriterionStates = map[string]StateRecord{}
	next.PhaseBlocks = map[string]StateRecord{}
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
	}
	return next
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
