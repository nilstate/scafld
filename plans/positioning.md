# scafld Positioning

scafld builds long-running AI coding work under adversarial review. This
plan owns audience, distribution, and how scafld is positioned relative to
the tools it sits above.

The internal architecture is covered in [llm-performance-cutover.md](llm-performance-cutover.md).
This doc is the external story.

## The Problem

Agents write code faster than humans can review it. Tests catch what tests
cover, which is rarely the thing that actually broke. Human reviewers
cannot keep up with agent-authored PRs once an agent is embedded in the
loop. And the agent grading its own work is the default failure mode: the
same model that produced the code says the code is fine, which is not
evidence.

The operational result is agent-authored code landing on main with silent
divergence from intent. Eventually something breaks in a way nobody can
trace to the moment it slipped in.

## The Mechanism

Three moves together:

- **Adversarial review gate.** A challenger agent, separate from the
  executor, reviews at the `review` gate and writes a verdict. The
  executor cannot ship unchallenged.
- **Spec as reviewed contract.** Work starts from a spec that states what
  must be true. The spec is reviewed before build begins, not after.
- **Session as durable ledger.** Every attempt, phase summary, challenger
  verdict, and human approval lives in `.ai/runs/{task-id}/session.json`.
  Recovery, telemetry, and audit all read from the same source.

The identity statement: the agent does not get to grade its own homework.

## Audience

**Primary.** Developers doing serious AI-assisted coding work. Concrete
shape:

- Using Claude Code, Cursor, Aider, Windsurf, Copilot Workspace, or
  equivalent for non-trivial tasks (not just autocomplete)
- Shipping agent-authored code to production, not just spike or scratch
  work
- Working on codebases where drift between intent and output actually
  costs
- Already feeling the pain of agents shipping broken or divergent work

**Secondary.** Teams with agent-authored PR pipelines. The scaling version
of the primary audience. Same pain, bigger blast radius.

**Not in scope.**

- Light-assist users who only use AI for autocomplete or rubber-duck
  debugging. The review gate has no cost-to-value fit for them.
- Teams that do not trust agents to write code at all. Different audience,
  different product.
- Users looking for a LangChain-specific tool. scafld is framework-agnostic.

## Positioning Relative to Other Tools

scafld sits **above** AI coding tools, not alongside them.

- **Not competing with** Claude Code, Cursor, Aider, Windsurf, Copilot
  Workspace. These are the agents. scafld wraps them.
- **Not the same as** LangSmith or AgentOps. Those are runtime
  observability for production agent apps. scafld is development-time
  discipline for agent-authored code.
- **Not a replacement for** tests, type checkers, or linters. Those live
  inside the spec and run during build. scafld's gate is whether the
  whole delivery matches the spec, which is a question tests cannot ask.
- **Closest conceptual neighbour.** Nothing direct. The category of
  "discipline wrapper for AI coding" is underformed. That is an
  opportunity, not a problem.

The one-line position: **"When agents write code for you, scafld makes
sure the code actually works."**

## What scafld Is Not

Explicit non-claims so positioning stays disciplined:

- Not a LangChain-specific integration or migration tool.
- Not runtime governance for agent applications.
- Not a code-migration engine that reads one framework and emits another.
- Not an alternative to any AI coding tool.
- Not a test runner.
- Not a CI system.
- Not an observability platform.

## Distribution

**Shipping today.** scafld is on PyPI and npm as of version 1.4.x. The CLI
works standalone. No runtime service required.

**Adoption gesture.** `scafld init` in any repo that wants the workflow.
Single command sets up `.ai/` directory. Works without any other
portfolio component.

**Composability.** scafld runs whatever coding agent the operator has. It
does not require a specific agent, framework, or runtime. That means:

- users running Claude Code can adopt scafld today
- users running Cursor can adopt scafld today
- users running their own agent can adopt scafld today
- users running multiple agents can adopt scafld today

**Integration hooks that would help adoption** (not shipped, prioritisation
separate):

- GitHub Action that runs scafld review on a PR
- Claude Code hook that invokes scafld at relevant lifecycle points
- Cursor extension or rule file that teaches Cursor to respect scafld phases
- VS Code extension equivalent

## Growth Levers

**Content.** Posts that name the problem concretely. "Your agent just shipped
a subtle bug and nobody caught it. Here's the review gate that would have."
Content works because the problem is visceral for the audience once named.

**Dogfooding receipts.** scafld is already used inside runx daily. The runx
team can publish specific case studies of regressions scafld caught.
Specific means "here is the challenger verdict that blocked the merge,"
not abstract case studies.

**Agent integrations.** A Claude Code hook or Cursor rule file that drops
scafld into someone's workflow in one command is the highest-leverage
distribution move. Adoption friction drops to near zero.

**Talks at AI-coding-adjacent venues.** The audience overlaps heavily with
communities already talking about Claude Code, Cursor, and agent coding
practice. scafld belongs in those conversations.

## Risks

1. **Review gate concept is unfamiliar.** "Adversarial review" sounds
   academic until someone has been bitten. Mitigation: lead with the
   concrete problem (agent shipped broken code) before the mechanism
   (challenger review).
2. **Users conflate with tests.** "I already have tests" is a natural first
   objection. Mitigation: position the challenger verdict as catching
   *spec-to-delivery* drift, which tests cannot catch by construction. Tests
   live in the spec; scafld verifies the spec was met.
3. **AI coding tools ship built-in review.** If Claude Code or Cursor adds
   adversarial review natively, scafld's edge narrows. Mitigation: scafld
   is agent-agnostic, works above any tool, and carries phase
   discipline plus session ledger that single-tool review features are
   unlikely to match. Also, if the category matures that way, scafld's
   early positioning becomes reinforcement, not competition.
4. **Too meta for current mainstream awareness.** Much of the AI coding
   audience is still in the honeymoon phase where agents feel magical.
   Adversarial review lands better once someone has been burned.
   Mitigation: position scafld where the already-burned users congregate,
   and let organic awareness catch up.

## Re-evaluation Triggers

- A major AI coding tool ships native adversarial review: reposition.
- Adoption stalls after initial dogfooding content: reconsider the
  distribution hooks, not the core pitch.
- A specific vertical (regulated-industry coding, high-stakes infra)
  shows unusual pull: consider a vertical-focused positioning variant.
- scafld usage inside runx uncovers a shape that generalises better than
  the current pitch: update the positioning, not just the implementation.

## What This Plan Is Not

- **Not a roadmap.** Engineering sequencing lives in scafld's other plans
  (currently [llm-performance-cutover.md](llm-performance-cutover.md)).
- **Not a marketing plan.** Concrete campaign work belongs in 0state's
  [outreach.md](../../0state/.plans/outreach.md) once scafld is pulled
  into that engine's target list.
- **Not a vertical pitch.** If vertical positioning becomes needed later
  (e.g. regulated-industry coding), it gets its own doc.

## Where This Fits in the Portfolio

scafld is its own product with its own audience. It is not a component of
runx, not a LangChain contender, not a sourcey add-on. It is the
discipline layer for serious AI coding work, independent of what runtime
or framework the coder uses.

scafld happens to be used inside runx daily. That is dogfooding, not
product dependency.

scafld may generate cross-promotion with runx (runx users who also code
with agents will find scafld useful). That is compounding, not coupling.
