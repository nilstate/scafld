---
spec_version: '2.0'
task_id: support-windows-receipt-reviewer
created: '2026-07-12T05:38:07Z'
updated: '2026-07-12T05:42:50Z'
status: review
harden_status: passed
size: small
risk_level: medium
---

# support-windows-receipt-reviewer

## Current State

Status: review
Current phase: final
Next: complete
Reason: review gate pass
Blockers: none
Allowed follow-up command: `scafld complete support-windows-receipt-reviewer`
Latest runner update: 2026-07-12T05:49:04Z
Review gate: pass

## Summary

Allow receipt-grade review finalization to validate native Windows reviewer
executables without weakening the absolute-path and hashing guarantees.

## Objectives

- Preserve the absolute-path, regular-file, and SHA-256 pinning requirements.
- Apply platform-native executable validation on Windows and Unix.
- Keep missing, relative, directory, and non-executable paths fail-closed.
- Prove the Windows behavior with the running Go test executable.

## Scope

- In scope: `internal/adapters/providers/env_scrub.go`, the focused receipt
  binary tests in `internal/adapters/providers/provider_test.go`, and the
  shared fake-executable helper in `evidence_sandbox_test.go`.
- Out of scope: provider selection, environment scrubbing, receipt schema,
  signing, review prompts, CLI flags, and unrelated finalize behavior.

## Dependencies

- Go standard library `os/exec` path validation.

## Assumptions

- `ResolveReceiptGradeBinary` continues to receive an absolute configured path.
- Go's `exec.LookPath` does not search PATH when the name contains a path
  separator, including an absolute path.

## Touchpoints

- `internal/adapters/providers/env_scrub.go`
- `internal/adapters/providers/provider_test.go`
- `internal/adapters/providers/evidence_sandbox_test.go`

## Risks

- Executability semantics differ by platform; use the standard library rather
  than reproducing Windows extension or Unix permission rules.
- A validation change could weaken fail-closed behavior; retain and run all
  relative, missing, directory, and non-executable rejection tests.

## Acceptance

Profile: standard

Validation:
- Focused receipt-grade binary tests.
- Complete providers package tests.
- Complete finalize adapter tests.
- Finalize application tests and repository-wide vet.
- Diff hygiene.

## Phase 1: Implementation

Status: pass
Dependencies: none

Objective: Make receipt-grade binary resolution accept a real native Windows

Changes:
- Replace the Unix-only permission-bit precheck with absolute-path `exec.LookPath` validation.
- On Windows, make the positive test resolve the running Go test executable; retain the current temporary executable fixture on Unix.
- Make the provider package's shared fake-executable helper use the running Go test executable on Windows.

Acceptance:
- [x] `ac1` command - Focused receipt-grade binary contract
  - Command: `go test ./internal/adapters/providers -run TestReceiptGradeBinary -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-26
- [x] `ac2` command - Provider adapter regression suite
  - Command: `go test ./internal/adapters/providers -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-27
- [x] `ac3` command - Finalize adapter regression suite
  - Command: `go test ./internal/adapters/cli/finalize -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-28
- [x] `ac4` command - Finalize application regression suite
  - Command: `go test ./internal/app/finalize -count=1`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-29
- [x] `ac5` command - Repository vet
  - Command: `go vet ./...`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-30
- [x] `ac6` command - Patch has no whitespace errors
  - Command: `git diff --check`
  - Expected kind: `exit_code_zero`
  - Status: pass
  - Evidence: exit code was 0
  - Source event: entry-31

## Rollback

- Revert `env_scrub.go` and `provider_test.go` together to restore the prior
  Unix permission-bit behavior.

## Review

Status: completed
Verdict: pass
Mode: discover
Provider: codex:gpt-5.5
Output: codex.output_file
Summary: No completion blockers found. The scoped change replaces Unix permission-bit validation with platform-native exec.LookPath validation while retaining exact absolute path hashing and fail-closed handling for invalid reviewer paths. Recorded acceptance evidence was treated as already executed per review instructions; no tests were rerun in this read-only review.

Attack log:
- `task-scoped diff`: source_diff_review -> clean (Reviewed the scoped diff for env_scrub.go, provider_test.go, and evidence_sandbox_test.go. The production change preserves absolute path, regular file, executable validation, and SHA-256 hashing; tests switch positive Windows fixtures to the running Go test executable.)
- `receipt-grade provider selection and runtime facts`: receipt_grade_call_chain -> clean (Traced ResolveReceiptGradeBinary through SelectReceiptGradeAgentWithEvidence and receiptGradeAgentProviderFor. The selected absolute binary path is hashed once, stamped into RuntimeFacts, and the same binary.Path is passed to the provider, with no later PATH substitution.)
- `ResolveReceiptGradeBinary fail-closed behavior`: fail_closed_cases -> clean (Checked tests and implementation for missing, relative, directory, and non-executable paths. ResolveReceiptGradeBinary trims input, rejects empty/relative paths, stats the exact path, rejects directories, uses exec.LookPath for native executable validation, and hashes only after validation.)
- `Windows reviewer executable test coverage`: windows_fixture_coverage -> clean (Checked Windows-specific test fixture changes in provider_test.go and evidence_sandbox_test.go. Positive receipt-grade paths now use os.Executable on Windows while Unix continues to use a temporary executable fixture; shared helper covers receipt-grade sandbox tests.)

Findings:
- none

## Self Eval

- none

## Deviations

- Replaced the repository-wide `go test ./...` criterion with the finalize
  application suite after the baseline repository produced unrelated Windows
  failures for Unix-only shell fixtures, path separators, Make, and executable
  mode assertions. Provider, CLI finalize, app finalize, repository vet, and
  diff-hygiene gates remain mandatory.

## Metadata

- created_by: scafld

## Origin

Created by: scafld
Source: plan

## Harden Rounds

### round-1

Status: passed
Started: 2026-07-12T05:38:33Z
Ended: 2026-07-12T05:38:54Z

Observations:
- design
  - Result: clean
  - Anchor: code:internal/adapters/providers/env_scrub.go:88
  - Note: The shared receipt-grade resolver owns absolute-path, executability,
- scope
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: The two-file scope covers the resolver and its focused tests without
- path
  - Result: clean
  - Anchor: code:internal/adapters/providers/provider.go:273
  - Note: Receipt-grade provider selection already funnels every configured
- command
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: Focused positive and rejection tests run before package, finalize,
- timing
  - Result: n/a
  - Anchor: code:internal/adapters/providers/env_scrub.go:88
  - Note: The resolver is synchronous preflight validation with no retry,
- rollback
  - Result: clean
  - Anchor: spec_gap:Rollback
  - Note: Resolver and test changes revert together; no persisted format or


## Planning Log

- none
