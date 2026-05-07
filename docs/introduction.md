---
title: Introduction
description: A deterministic protocol for multi-phase agent work
---

# Introduction

**A deterministic protocol for multi-phase agent work.**
The agent passes through. The protocol stays.

Plans outlive agents. Sessions hold the receipts. Reviews take nothing on faith.

scafld gives AI coding work a deterministic state machine. Every non-trivial
task starts as a Markdown living spec. Every important runtime event appends to
the session ledger. Current state, next command, acceptance evidence, and review
gate are derived from those two artifacts instead of inferred from chat.

That is the core product shape: the agent can pass through, restart, or be
replaced. The protocol stays. Auditability falls out of the model because the
same evidence that explains what happened is also what drives what happens next.

The spec declares what will change, why it will change, what constitutes
success, and what must not break. The agent executes against the spec, not
against a loose prompt. When the work is done, scafld validates acceptance
evidence and sends the result through adversarial review.

## The core idea

A spec is a contract between the human who understands the problem and the agent that will write the code. It captures:

- **Objectives** - what this task must accomplish
- **Scope** - what files change, what stays untouched
- **Phases** - ordered steps with individual acceptance criteria
- **Invariants** - project-wide rules that must never break
- **Acceptance criteria** - concrete, executable validation

The spec lives in version control alongside the code. It moves through a
lifecycle: draft, approved, active, review, completed. The filesystem is the
state machine; directories represent states, and the session ledger records the
evidence behind each transition.

## Agent-facing deterministic gates

Each gate exposes the same repair contract:

- trusted state
- failure reason
- evidence path
- expected shape
- allowed next command

scafld is strict in what it trusts and generous in what it explains. If a gate
blocks, the next agent should not have to infer the blocker from stale prose,
raw diagnostics, or unrelated workspace dirt.

## Agent-agnostic

scafld doesn't care which agent writes the code. Claude Code, Cursor, Copilot, a custom pipeline; the spec is the interface. The agent reads the spec, does the work, and scafld validates the result. If you switch agents mid-project, the specs carry over unchanged.

## What scafld is not

scafld is not a project management tool. It doesn't replace your issue tracker or sprint board. It operates at a lower level: the boundary between "what should happen" and "make it happen." It's the engineering discipline layer that sits between human intent and agent execution.

scafld is not a code generator. It doesn't write code. It ensures that whatever writes the code; human or agent; does so against a well-defined specification with verifiable outcomes.

## When to use it

Use scafld when a task is complex enough that you'd regret not having written it down first. The threshold varies by project, but a good heuristic: if the task touches more than two files or requires understanding context beyond the immediate change, it deserves a spec.

Micro tasks (rename a variable, fix a typo) don't need specs. Everything else probably does.
