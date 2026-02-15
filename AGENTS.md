# AGENTS.md

Repository-level policy for Codex and sub-agents.

## Default Behavior (Automatic)

For any thread in this repository, agents must apply these rules by default.
You do not need to explicitly tell agents to use these files each time.

## Source of Truth Order

1. `docs/project-spec.md` for product scope, architecture, timeline, and acceptance criteria.
2. `docs/week-N-execution.md` for the active week plan and live status.
3. `docs/agent/workflow.md` for execution workflow and decision rules.
4. `docs/agent/handoff-template.md` for handoff/report format.
5. `AGENTS.md` for top-level policy.

If there is any conflict:
- Product/timeline conflict: `docs/project-spec.md` wins.
- Week execution conflict: active `docs/week-N-execution.md` wins.

## Mandatory Rules

- Clarification before assumption:
  Ask the user before making any assumption that could affect scope, architecture, timeline, cost, security, environment, or data contracts.
- Scope discipline:
  Execute only the requested week/milestone scope unless the user explicitly approves scope expansion.
- Weekly status discipline:
  Keep exactly one execution file per active week (`docs/week-1-execution.md`, `docs/week-2-execution.md`, etc.).
  Update that file's live status sections before closing a thread.
- Single-task sequencing discipline:
  Execute one task ID at a time and wait for review/confirmation before moving to the next task ID.
  Do not auto-batch all tasks for a day unless the user explicitly requests batching.
- Code observability discipline:
  Every new or modified function must include a short purpose comment and meaningful logging for key state transitions and error paths.
- README runbook discipline:
  Keep `README.md` as a single cumulative runbook for the latest completed day (Day N).
  When Day N is added, remove/replace Day (N-1) step sections so instructions only reflect setup/test flow after Day N.
- Context hygiene discipline:
  Follow `docs/agent/workflow.md` context-hygiene rules to minimize prompt bloat automatically.
  Agents own this upkeep; user prompting is not required.

## Required Read Order for New Execution Threads

1. `docs/project-spec.md`
2. Active `docs/week-N-execution.md`
3. `docs/agent/workflow.md`
4. `docs/agent/handoff-template.md`

Read in minimal mode defined by `docs/agent/workflow.md` (do not load unnecessary sections).

## Change Management

- If product scope/timeline changes, update `docs/project-spec.md` change log.
- If workflow/handoff behavior changes, update `docs/agent/workflow.md` and/or `docs/agent/handoff-template.md`.
- Keep `AGENTS.md` short; only policy belongs here.
