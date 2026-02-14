# AGENTS.md

Repository coordination guide for Codex and other sub-agents.

## Purpose

Use this file to define **how agents should work** in this repo.
Use `docs/project-spec.md` to define **what to build** and **when**.

## Source of Truth

1. `docs/project-spec.md` is the canonical product/architecture/timeline spec.
2. `AGENTS.md` defines execution behavior, guardrails, and handoff rules.
3. If they conflict on product scope/timeline, treat `docs/project-spec.md` as authoritative and update `AGENTS.md` accordingly.

## Default Thread Policy (No Pasted Opener Needed)

For any thread started in this repository, agents should assume the following by default:

1. Follow this `AGENTS.md` workflow automatically.
2. Use `docs/project-spec.md` as the canonical scope/timeline reference.
3. Execute only the requested week/milestone scope from `docs/project-spec.md`.
4. Validate and report against that week's acceptance criteria before closing.

Only override these defaults when the user explicitly requests a different scope or process.

## Required Read Order for New Threads

1. Read `docs/project-spec.md`.
2. Read this `AGENTS.md`.
3. Read thread-specific task docs (if present), for example:
   - `docs/week-1-execution.md`
   - `docs/tickets/*.md`

## Execution Status Storage (Minimal Files)

To keep handoffs smooth without creating many files:

1. Keep exactly one execution file per active week:
   - `docs/week-1-execution.md`
   - `docs/week-2-execution.md`
   - `docs/week-3-execution.md`
   - `docs/week-4-execution.md`
2. Do not create separate weekly status files unless explicitly requested.
3. Use each week's execution file as the live status/handoff artifact by updating:
   - `Live Task Status` table
   - `Session Log (append-only)`
   - `Handoff Snapshot`
4. At the end of each working session/thread, update the current week's execution file before closing.
5. At week transition, create the next `docs/week-N-execution.md` and carry forward only unresolved risks/blockers.

## Working Agreement

- Keep high-level planning in a strategy thread.
- Keep implementation details in execution threads.
- Before coding, restate: scope, current week/milestone, and acceptance criteria.
- Do not implement out-of-scope features unless explicitly approved.

## Week-Based Execution Rules

- Week 1: Local Docker Compose core flow (temporary REST allowed).
- Week 2: GraphQL + gRPC + subscriptions migration.
- Week 3: Azure provisioning/deployment + Jenkins CI/CD.
- Week 4: Monitoring, hardening, and final docs.

Always verify the current week goals against `docs/project-spec.md` before starting.

## Change Management

- Any change to scope, architecture, or timeline must be recorded in:
  - `docs/project-spec.md` change log section.
- If execution process changes (workflow, conventions, handoff rules), update `AGENTS.md`.

## Handoff Template (Use in PRs or Thread Updates)

- Scope: (what milestone/week item was targeted)
- Changes: (files/services touched)
- Acceptance criteria status: (pass/fail per criterion)
- Risks/issues: (blockers, tradeoffs, follow-ups)
- Next step: (smallest actionable next task)

## Practical Usage With docs/project-spec.md

- Put decisions and milestones in `docs/project-spec.md`.
- Put agent operating rules in `AGENTS.md`.
- Linking both in new execution threads is optional, because default thread policy applies automatically.

Optional short thread opener:

```text
Execute Week 1 from docs/week-1-execution.md.
```
