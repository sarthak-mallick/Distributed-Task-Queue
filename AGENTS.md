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
- Workflow authority discipline:
  Treat `docs/agent/workflow.md` as the canonical location for execution-level rules (task sequencing and batching preferences, code observability, README/runbook maintenance, current E2E runner maintenance, and context hygiene).
  Avoid duplicating those detailed rules in `AGENTS.md`.

## Thread Start Reference

Use the thread-start checklist and minimal-read mode from `docs/agent/workflow.md` as the canonical process.

## Change Management

- If product scope/timeline changes, update `docs/project-spec.md` change log.
- If workflow/handoff behavior changes, update `docs/agent/workflow.md` and/or `docs/agent/handoff-template.md`.
- Keep `AGENTS.md` short; only policy belongs here.
