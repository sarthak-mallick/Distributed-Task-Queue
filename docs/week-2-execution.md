# Week 2 Execution Brief

Last updated: 2026-02-14  
Canonical reference: `docs/project-spec.md` (Week 2 section + acceptance criteria)

## Objective

Migrate local system from temporary REST to GraphQL + subscriptions and add gRPC API-worker communication.

## In Scope (Week 2 Only)

- Replace temporary REST API with GraphQL
- Implement GraphQL queries, mutations, subscriptions
- Add gRPC service contracts between API and worker
- React Apollo Client integration with WebSocket subscriptions
- Real-time updates without polling

## Out of Scope

- AKS/Ansible/Jenkins deployment work (Week 3)
- Prometheus/Grafana hardening work (Week 4)

## Ticket-Ready Task List

| ID | Task | Estimate | Depends On | Done When |
|----|------|----------|------------|-----------|
| W2-01 | Define GraphQL schema and resolver contract | 0.5d | Week 1 complete | Schema covers submit/status/progress subscription |
| W2-02 | Implement GraphQL mutation/query path in API | 1d | W2-01 | Job submit/status flows work via GraphQL |
| W2-03 | Implement GraphQL subscriptions over WebSocket | 1d | W2-01 | Live progress events delivered without polling |
| W2-04 | Define gRPC proto between API and worker | 0.5d | Week 1 complete | Proto contract finalized and versioned |
| W2-05 | Implement gRPC server/client wiring | 1d | W2-04 | API-worker internal calls use gRPC path |
| W2-06 | Migrate React to Apollo Client + subscriptions | 1d | W2-02, W2-03 | UI uses GraphQL only and renders live updates |
| W2-07 | Remove/disable temporary REST endpoints | 0.5d | W2-02 | REST no longer used in normal flow |
| W2-08 | Integration test + documentation updates | 0.5d | W2-07 | Week 2 acceptance checklist green |

## Live Task Status

Update this table continuously during Week 2 execution.

| ID | Status | Last Updated | Notes |
|----|--------|--------------|-------|
| W2-01 | Not Started | 2026-02-14 | - |
| W2-02 | Not Started | 2026-02-14 | - |
| W2-03 | Not Started | 2026-02-14 | - |
| W2-04 | Not Started | 2026-02-14 | - |
| W2-05 | Not Started | 2026-02-14 | - |
| W2-06 | Not Started | 2026-02-14 | - |
| W2-07 | Not Started | 2026-02-14 | - |
| W2-08 | Not Started | 2026-02-14 | - |

## Week 2 Acceptance Checklist

- Job submission/status retrieval works via GraphQL.
- Live progress updates are delivered via GraphQL subscriptions (no polling).
- API-worker communication uses gRPC contracts for internal calls.
- Temporary REST endpoints are removed or disabled.
- End-to-end local flow remains functional after migration.

## Session Log (Append-Only)

- 2026-02-14: Week 2 execution brief created as next-week handoff target.

## Handoff Snapshot

- Week status: Not started
- Completed tasks: None
- In-progress tasks: None
- Blockers: None
- Next task: W2-01 define GraphQL schema and resolver contract
