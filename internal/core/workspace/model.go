package workspace

// InitResult describes the workspace paths created during bootstrap.
type InitResult struct {
	Root    string   `json:"root"`
	Created []string `json:"created"`
}
