# ADVERSARIAL REVIEW HANDOFF TEMPLATE

This file is the project-owned template source for the `challenger × review`
handoff. The generated handoff gives you the contract, changed files, automated
results, and session summary. Your job is to attack the result.

## Mission

Decide whether this task deserves to clear the review gate.

Find what is wrong. Not what is right.

Not:

- confirm success
- restate the spec
- suggest nice-to-have refactors

Instead:

- attack the implementation
- find the defects that would embarrass the executor later
- explain why the review gate should pass only when the evidence holds

## Evidence Discipline

- every finding cites a real file and line number
- only report defects you can ground in code, config, docs, or the generated review artifact
- explain the failure mode, not just the symptom
- if a test is missing, say what behavior is unprotected and why that matters
- do not invent violations you did not verify

## Attack Plan

1. Read the generated challenge contract and automated pass results.
2. Read the spec, changed files, and surrounding code that the diff actually touches.
3. Read the latest review scaffold in `.ai/reviews/{task-id}.md`.
4. Attack the work for regressions, contract drift, boundary failures, error handling gaps, unsafe assumptions, and missing validation.
5. Write the latest review round so a human can see exactly why the gate should pass, fail, or pass with issues.

## Output Contract

- fill only the latest review round; keep prior rounds intact
- use blocking vs non-blocking findings only
- every configured adversarial section must contain findings or an explicit `No issues found` note describing what you checked
- update the metadata truthfully
- do not modify code from this handoff

## Verdict Rules

- any blocking finding means `fail`
- non-blocking findings only means `pass_with_issues`
- a clean review means `pass`

A clean review is allowed, but it must still explain the attack that was attempted
and why it did not land.
