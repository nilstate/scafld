---
title: Introduction
description: Spec-driven orchestration for AI coding agents
---

# Introduction

Every serious engineering discipline separates planning from execution. Civil engineers don't wing it at the construction site. Aerospace engineers don't improvise turbine blade geometry. The separation exists because complex systems fail when the person holding the tool is also deciding what to build.

AI coding agents have no such separation. You type a prompt, the agent writes code, and you hope the result matches what you meant. For trivial tasks this works. For anything that touches multiple files, crosses module boundaries, or carries real risk; it doesn't. The agent hallucinates scope, drifts from objectives, and produces code that technically compiles but misses the point.

scafld enforces the separation. Every non-trivial task becomes a YAML specification before a single line of code changes. The spec declares what will change, why it will change, what constitutes success, and what must not break. The agent executes against the spec, not against a loose prompt. When the work is done, scafld validates the result against the spec's acceptance criteria and audits for scope drift.

## The core idea

A spec is a contract between the human who understands the problem and the agent that will write the code. It captures:

- **Objectives** - what this task must accomplish
- **Scope** - what files change, what stays untouched
- **Phases** - ordered steps with individual acceptance criteria
- **Invariants** - project-wide rules that must never break
- **Acceptance criteria** - concrete, executable validation

The spec lives in version control alongside the code. It moves through a lifecycle: draft, review, approved, in-progress, review, completed. The filesystem is the state machine; directories represent states.

## Agent-agnostic

scafld doesn't care which agent writes the code. Claude Code, Cursor, Copilot, a custom pipeline; the spec is the interface. The agent reads the spec, does the work, and scafld validates the result. If you switch agents mid-project, the specs carry over unchanged.

## What scafld is not

scafld is not a project management tool. It doesn't replace your issue tracker or sprint board. It operates at a lower level: the boundary between "what should happen" and "make it happen." It's the engineering discipline layer that sits between human intent and agent execution.

scafld is not a code generator. It doesn't write code. It ensures that whatever writes the code; human or agent; does so against a well-defined specification with verifiable outcomes.

## When to use it

Use scafld when a task is complex enough that you'd regret not having written it down first. The threshold varies by project, but a good heuristic: if the task touches more than two files or requires understanding context beyond the immediate change, it deserves a spec.

Micro tasks (rename a variable, fix a typo) don't need specs. Everything else probably does.
