# Agent Workflow

Operational workflow for implementation threads in this repository.

## 1) Thread Start Checklist

1. Read `docs/project-spec.md`.
2. Read active `docs/week-N-execution.md`.
3. Confirm scope: current week, tasks, and acceptance criteria.
4. If any critical detail is unclear, ask clarification questions before implementation.

Use minimal-read mode for `docs/week-N-execution.md`:
1. `Objective`
2. `In Scope` / `Out of Scope`
3. `Live Task Status`
4. `Handoff Snapshot`
5. Latest session-log entries (most recent only)
6. Only the task/contract subsection needed for current work

## 2) Clarification Triggers (Ask First)

Ask the user before proceeding when any of these are ambiguous:
- Contract details (schema fields, queue/topic names, API behavior).
- Implementation direction with meaningful tradeoffs.
- Changes to acceptance criteria, task ordering, or timeline.
- New external service/dependency, cost, security, or infra impact.

Question style:
- Keep questions short and decision-focused.
- Offer options when helpful.
- Wait for answer before assuming.

## 3) Execution Rules

- Stay within requested week scope.
- Use the active week file as the live source for task status.
- Do not create separate status files unless requested.
- Keep changes incremental and tied to specific task IDs.

## 4) Week Status Update Rules

During execution, update active `docs/week-N-execution.md`:
- `Live Task Status`
- `Session Log (Append-Only)`
- `Handoff Snapshot`

Update points:
- When a task starts.
- When a task completes/fails/blocks.
- Before thread close.

Status entry style:
- Keep entries short and factual (1-2 lines).
- Prefer task IDs and outcome over long narrative.

## 5) End-of-Thread Checklist

1. Verify/report acceptance criteria progress.
2. Record blockers/risks and unresolved decisions.
3. Set next smallest actionable task.
4. Update `Handoff Snapshot`.
5. Use `docs/agent/handoff-template.md` format in final summary.

## 6) Week Transition Rule

When moving from Week N to Week N+1:
- Create `docs/week-(N+1)-execution.md` if missing.
- Carry only unresolved blockers/risks from prior week.
- Do not copy stale completed-task noise.

## 7) Context Hygiene (Automatic)

Agents must keep context size controlled without user prompting:

1. Do not reread full weekly docs on every turn; reread only sections needed for the current task.
2. Keep `Session Log` concise; avoid pasting long command output or long prose.
3. Keep `Handoff Snapshot` current so new threads can start from it instead of full-history reading.
4. When a week is completed, compact the weekly file by:
   - preserving final acceptance status, key decisions, and unresolved risks
   - keeping a brief completion summary
   - condensing stale in-progress noise outside append-only logs
5. If additional detail is ever required, rely on git history rather than expanding execution docs indefinitely.
