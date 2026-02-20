# Week 4 Execution Brief (Single Source of Truth)

Last updated: 2026-02-19
Canonical product/timeline reference: `docs/project-spec.md` (Week 4 section + acceptance criteria)

This file is the active Week 4 execution and status artifact.

## Objective

Complete Week 4 monitoring, hardening, and runbook polish with a Day 17-first sequence that keeps the canonical local e2e flow current.

## Alignment Notes

- Week 3 deployment foundations are complete; Week 4 starts operational readiness scope.
- Scope follows canonical Week 4 goals only:
  - Deploy Prometheus and Grafana to AKS
  - Instrument Go services with `/metrics`
  - Add worker reliability and graceful shutdown hardening
  - Finalize docs/runbook polish

## In Scope (Week 4 Only)

- Metrics instrumentation in API and worker
- AKS monitoring manifests (Prometheus + Grafana)
- Worker resilience/shutdown hardening
- Runbook and handoff-document final polish

## Out of Scope

- New product features unrelated to monitoring/hardening
- Scope expansion into auth/tenancy redesign
- New cloud providers or alternate orchestration platforms

## Ticket-Ready Task List

| ID | Task | Estimate | Depends On | Done When |
|----|------|----------|------------|-----------|
| W4-001 | Add API + worker metrics instrumentation (`/metrics`) | 1d | Week 3 complete | Both services expose Prometheus-format metrics for core throughput/error/duration signals |
| W4-002 | Add AKS monitoring stack manifests (Prometheus + Grafana) | 1d | W4-001 | Monitoring manifests render cleanly and scrape app metrics endpoints |
| W4-003 | Improve worker reliability and graceful shutdown handling | 1d | W4-001 | In-flight processing and shutdown transitions are safe and observable |
| W4-004 | Final docs/runbook polish and acceptance closure | 1d | W4-001, W4-002, W4-003 | Week 4 acceptance criteria are documented with validation evidence |

## Live Task Status

| ID | Status | Last Updated | Notes |
|----|--------|--------------|-------|
| W4-001 | Completed | 2026-02-19 | Day 17 completed: API + worker expose Prometheus `/metrics`; runtime counters validated in canonical e2e. |
| W4-002 | Completed | 2026-02-19 | Day 17 completed: AKS monitoring manifests for Prometheus/Grafana added and render checks integrated. |
| W4-003 | Pending | 2026-02-19 | Planned for Day 18/19 hardening slice. |
| W4-004 | Pending | 2026-02-19 | Planned for Week 4 closure after hardening tasks. |

## Implementation Order

1. Day 17: W4-001 and baseline W4-002.
2. Day 18-19: W4-003 hardening and reliability.
3. Day 20: W4-004 final acceptance and docs closure.

## Day-by-Day Plan

### Day 17 - Monitoring Foundations
- Implement `/metrics` endpoints and core counters in API/worker.
- Add AKS monitoring manifests and render checks.
- Extend canonical `scripts/run-current-e2e.sh` checks for monitoring artifacts and metrics endpoints.

### Day 18 - Worker Reliability Hardening
- Strengthen retry/error-path handling and shutdown observability.

### Day 19 - Graceful Shutdown Safety
- Validate in-flight job behavior and shutdown sequencing in worker.

### Day 20 - Final Polish
- Finalize docs/runbook and Week 4 acceptance evidence.

## Week 4 Acceptance Checklist

- Dashboards show throughput, error rate, duration, and worker health.
- Metrics and alerts support operational troubleshooting.
- In-flight jobs are handled safely during worker shutdown.
- Project docs are complete enough for new contributors to run and operate.

## Session Log (Append-Only)

- 2026-02-19: Created Week 4 execution file and started Day 17 monitoring foundations (`W4-001`, `W4-002`).
- 2026-02-19: Implemented API/worker metrics instrumentation and AKS monitoring manifests; updated canonical e2e to validate monitoring artifacts and runtime metrics counters.
- 2026-02-19: Validation evidence:
  - `go test ./...` (api) passed
  - `go test ./...` (worker) passed
  - `bash scripts/run-current-e2e.sh --with-ui-checks --purge` passed (run id `20260219224508`)
- 2026-02-19: Re-validated after canonical script day17 tag cleanup:
  - `bash scripts/run-current-e2e.sh --with-ui-checks --purge` passed (run id `20260219224642`)

## Handoff Snapshot

- Week status: In progress (Day 17 complete)
- Completed tasks: W4-001, W4-002
- In-progress tasks: None
- Blockers: None
- Next task: Start Day 18 hardening work for `W4-003` (worker reliability and graceful shutdown safety).
