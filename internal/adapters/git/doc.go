// Package git inspects workspace state through Git-backed adapters.
//
// Snapshot builds a commit-free fingerprint by writing the working tree through
// a temporary GIT_INDEX_FILE, seeded from HEAD when available, and then
// resolving a Git tree object. The real branch, index, HEAD, and stash are not
// mutated. Snapshot commands pin host core.autocrlf=false while preserving
// committed .gitattributes and filters as repository semantics.
//
// Submodules are represented as gitlink entries: the snapshot records the
// opaque submodule commit pointer, not the nested submodule working tree.
package git
