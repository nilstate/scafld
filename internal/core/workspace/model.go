package workspace

// InitResult describes the workspace paths created during bootstrap.
type InitResult struct {
	Root    string   `json:"root"`
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
}

// Merge appends another installer result into the workspace bootstrap result.
func (r *InitResult) Merge(created []string, updated []string, skipped []string) {
	r.Created = append(r.Created, created...)
	r.Updated = append(r.Updated, updated...)
	r.Skipped = append(r.Skipped, skipped...)
}

// Changed reports whether bootstrap created or updated any workspace files.
func (r InitResult) Changed() bool {
	return len(r.Created) > 0 || len(r.Updated) > 0
}
