// Package agentcontract models role contracts delivered to agents.
package agentcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
)

// Role names one agent-facing contract.
type Role string

const (
	RolePlan     Role = "plan"
	RoleHarden   Role = "harden"
	RoleBuild    Role = "build"
	RoleReview   Role = "review"
	RoleRecovery Role = "recovery"
)

// Roles returns every managed role contract.
func Roles() []Role {
	return []Role{RolePlan, RoleHarden, RoleBuild, RoleReview, RoleRecovery}
}

// Filename returns the managed prompt filename for role.
func (r Role) Filename() string {
	return string(r) + ".md"
}

// Valid reports whether role is one of the managed contract roles.
func (r Role) Valid() bool {
	for _, candidate := range Roles() {
		if r == candidate {
			return true
		}
	}
	return false
}

// Contract is a source-backed role contract resolved by an adapter.
type Contract struct {
	Role   Role   `json:"role"`
	Path   string `json:"path"`
	Body   string `json:"body"`
	SHA256 string `json:"sha256"`
	Bytes  int    `json:"bytes"`
}

// New returns a normalized contract with digest provenance.
func New(role Role, path string, body []byte) (Contract, error) {
	if !role.Valid() {
		return Contract{}, fmt.Errorf("unknown agent contract role %q", role)
	}
	sum := sha256.Sum256(body)
	return Contract{
		Role:   role,
		Path:   strings.TrimSpace(path),
		Body:   strings.TrimSpace(string(body)),
		SHA256: hex.EncodeToString(sum[:]),
		Bytes:  len(body),
	}, nil
}

// Empty reports whether the contract has no body.
func (c Contract) Empty() bool {
	return strings.TrimSpace(c.Body) == ""
}

// Section converts the contract into a required review context section.
func (c Contract) Section(key string, title string, order int) reviewcontext.Section {
	if c.Empty() {
		return reviewcontext.Section{}
	}
	path := strings.TrimSpace(c.Path)
	if path == "" {
		path = ".scafld/core/prompts/" + c.Role.Filename()
	}
	return reviewcontext.Section{
		Key:      key,
		Title:    title,
		Order:    order,
		Body:     strings.TrimSpace(c.Body),
		Required: true,
		Sources: []reviewcontext.Source{{
			Kind:   "agent_contract",
			Path:   path,
			SHA256: c.SHA256,
			Bytes:  c.Bytes,
		}},
	}
}
