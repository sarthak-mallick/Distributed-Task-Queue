# Week 2 Execution Brief (Single Source of Truth)

Last updated: 2026-02-18  
Canonical product/timeline reference: `docs/project-spec.md` (Week 2 section + acceptance criteria)

This file is the active Week 2 execution and status artifact.

## Objective

Migrate local system from Week 1 REST + polling behavior to Week 2 GraphQL + subscriptions with API-worker gRPC communication.

## Alignment Notes

- Stays within canonical Week 2 scope from `docs/project-spec.md`:
  - GraphQL query/mutation/subscription path
  - gRPC API-worker internal communication
  - React Apollo + WebSocket integration
  - Week 1 functional behavior preserved during migration
- Keeps existing Week 1 data contracts for Kafka/Redis/Mongo compatible.
- Introduces explicit Week 2 contracts for GraphQL and gRPC under `contracts/`.

## In Scope (Week 2 Only)

- Replace temporary REST API surface with GraphQL endpoint(s)
- Add GraphQL subscription transport over WebSocket
- Implement API-worker gRPC contract and status/progress service path
- Migrate React UI to Apollo Client + GraphQL WebSocket subscriptions
- Validate migration acceptance and update runbook/e2e flow

## Out of Scope

- AKS/Ansible/Jenkins deployment work (Week 3)
- Prometheus/Grafana hardening work (Week 4)
- Auth/tenancy changes outside canonical scope

## Ticket-Ready Task List

| ID | Task | Estimate | Depends On | Done When |
|----|------|----------|------------|-----------|
| W2-001 | Define Week 2 GraphQL contract (`submitJob`, `jobStatus`, `jobProgress`) and API resolver mapping | 0.5d | Week 1 complete | Contract documented and resolver input/output fields finalized |
| W2-002 | Add API GraphQL HTTP endpoint for mutation/query path | 1d | W2-001 | Job submit + status retrieval work via GraphQL JSON requests |
| W2-003 | Add GraphQL WebSocket subscription endpoint | 1d | W2-001 | `jobProgress` events stream without client polling |
| W2-004 | Define and version API-worker gRPC contract | 0.5d | Week 1 complete | Proto and message/service contract committed under `contracts/grpc` |
| W2-005 | Implement worker gRPC status/progress server | 1d | W2-004 | Worker serves status query and progress stream via gRPC |
| W2-006 | Implement API gRPC client integration to worker | 1d | W2-004, W2-005 | GraphQL status/subscription resolvers use gRPC worker path |
| W2-007 | Disable Week 1 REST job endpoints from canonical path | 0.5d | W2-002, W2-006 | REST submission/status endpoints return disabled/removed behavior |
| W2-008 | Migrate React UI to Apollo Client + GraphQL subscriptions | 1d | W2-002, W2-003 | UI submits/query/subscribes over GraphQL only |
| W2-009 | Migration validation + docs/runbook/e2e update | 0.5d | W2-007, W2-008 | README + e2e script verify Week 2 acceptance criteria |

## Live Task Status

Update this table continuously during Week 2 execution.

| ID | Status | Last Updated | Notes |
|----|--------|--------------|-------|
| W2-001 | Completed | 2026-02-18 | GraphQL operation contract finalized for `submitJob`, `jobStatus`, `jobProgress` with Week 1-compatible payload/status fields. |
| W2-002 | Completed | 2026-02-18 | API now serves GraphQL HTTP mutation/query path at `POST /graphql`. |
| W2-003 | Completed | 2026-02-18 | API now serves GraphQL WebSocket subscriptions at `GET /graphql/ws` (`graphql-transport-ws`). |
| W2-004 | Completed | 2026-02-18 | Added `contracts/grpc/api-worker-v1.proto` as Week 2 API-worker contract baseline. |
| W2-005 | Completed | 2026-02-18 | Worker now exposes gRPC status/progress service (`GetJobStatus`, `SubscribeJobProgress`). |
| W2-006 | Completed | 2026-02-18 | API status/query/subscription path now calls worker via gRPC client. |
| W2-007 | Completed | 2026-02-18 | Week 1 REST job endpoints are disabled and return `410 Gone`. |
| W2-008 | Completed | 2026-02-18 | UI migrated to Apollo Client + GraphQL WebSocket subscriptions (no polling loop). |
| W2-009 | Completed | 2026-02-18 | `README.md`, service READMEs, and `scripts/run-current-e2e.sh` updated; full e2e passed. |

## Implementation Order

1. Establish contracts first (`W2-001`, `W2-004`).
2. Build worker gRPC server + API gRPC client path (`W2-005`, `W2-006`).
3. Build GraphQL HTTP + WS endpoints in API (`W2-002`, `W2-003`).
4. Disable REST job endpoints (`W2-007`).
5. Migrate UI to Apollo + subscriptions (`W2-008`).
6. Run migration acceptance checks and update docs/e2e (`W2-009`).

## Day-by-Day Plan

### Day 8 - Contracts + gRPC/GraphQL backend foundation
- W2-001, W2-004, W2-005, W2-006 (initial integration)
- Start W2-002/W2-003 API endpoint wiring

### Day 9 - Complete GraphQL API + subscription path
- Finish W2-002 and W2-003
- Execute W2-007 REST disablement

### Day 10 - UI Apollo/WebSocket migration
- Execute W2-008
- Validate manual end-to-end from UI

### Day 11 - Migration validation and docs
- Execute W2-009
- Confirm acceptance criteria and update handoff snapshot

## Week 2 Acceptance Checklist

- Job submission works via GraphQL mutation.
- REST submission/status paths are removed or disabled from canonical flow.
- Live progress updates are delivered via GraphQL subscriptions.
- API-worker communication uses Week 2 gRPC contract.
- Week 1 behavior remains functional after migration.

## Session Log (Append-Only)

- 2026-02-14: Placeholder Week 2 brief created for handoff.
- 2026-02-18: Week 2 activated; ticket list/day plan refreshed from canonical Week 2 goals.
- 2026-02-18: Started backend migration batch for GraphQL endpoints + gRPC API-worker integration.
- 2026-02-18: Implemented API GraphQL HTTP/WS path, worker gRPC service, and API gRPC client integration.
- 2026-02-18: Migrated React UI to Apollo mutation/query/subscription flow and removed polling path.
- 2026-02-18: Updated runbook/e2e contracts and validated with `bash scripts/run-current-e2e.sh --with-ui-checks --purge` (pass).
- 2026-02-18: Completed Day 9 stabilization pass with added GraphQL/WS and gRPC validation tests; re-ran full e2e successfully.

## Handoff Snapshot

- Week status: In progress (through Day 9 complete)
- Completed tasks: W2-001 through W2-009
- In-progress tasks: None
- Blockers: None
- Next task: Await user direction for Day 10+ scope (Week 2 core migration is already fully green)
