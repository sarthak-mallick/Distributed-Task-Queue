# Week 1 Execution Brief (Single Source of Truth)

Last updated: 2026-02-14  
Canonical product/timeline reference: `docs/project-spec.md` (Week 1 section + acceptance criteria)

This file is the only Week 1 execution and status artifact. It supersedes `docs/week1-execution-plan.md`.

## Objective

Deliver a fully working local end-to-end flow using Docker Compose:
- Submit weather job
- Queue/process job
- Track progress
- Persist final result
- View status in UI

## Alignment Notes

- Uses canonical Week 1 duration: 7 days.
- Uses canonical service skeleton names: `api`, `worker`, `ui`, `infra/compose`.
- Uses canonical Week 1 contract baselines:
  - Kafka required fields: `job_id`, `job_type`, `submitted_at`, `payload`, `trace_id`
  - Redis key: `job:{job_id}:status`; fields: `state`, `progress_percent`, `updated_at`, `message`
  - Mongo collection: `job_results`; required fields per spec
  - RabbitMQ request/reply required fields per spec, with AMQP `correlation_id` + `reply_to`
- Includes Day 6 stability work and Day 7 demo buffer.

## In Scope (Week 1 Only)

- Docker Compose infra: Kafka (KRaft), RabbitMQ, Redis, MongoDB
- Temporary Go REST API
- Go worker consuming Kafka and running weather jobs
- RabbitMQ request-reply progress path with correlation IDs
- Basic React UI: submit + status/progress

## Out of Scope

- GraphQL, subscriptions, Apollo WS (Week 2)
- gRPC API-worker path (Week 2)
- AKS/Ansible/Jenkins (Week 3)
- Prometheus/Grafana hardening work (Week 4)

## Proposed Repo Layout (Week 1)

- `api/` (Go temporary REST API for submit/progress)
- `worker/` (Go Kafka consumer + RabbitMQ progress responder)
- `ui/` (React submit + status)
- `infra/compose/` (Docker Compose stack, env, helper scripts)
- `contracts/` (JSON schemas + examples)
- `docs/` (quickstart, test checklist, demo checklist)

## Ticket-Ready Task List

| ID | Task | Estimate | Depends On | Done When |
|----|------|----------|------------|-----------|
| W1-001 | Create Week 1 repo skeleton (`api`, `worker`, `ui`, `infra/compose`, `contracts`, `docs`) | 2h | - | Directories and starter READMEs exist; paths match canonical spec |
| W1-002 | Compose infra: Kafka (KRaft), RabbitMQ, Redis, MongoDB + health checks + volumes | 6h | W1-001 | `docker compose up -d` succeeds; all services healthy; restart preserves data |
| W1-003 | Inter-service connectivity smoke scripts (Kafka/RabbitMQ/Redis/Mongo reachability) | 3h | W1-002 | One-command smoke test passes from clean startup |
| W1-004 | Define and commit canonical Week 1 contracts (Kafka, Redis, Mongo, RabbitMQ) | 4h | W1-001 | Contracts committed in `contracts/`; required fields match `project-spec.md` |
| W1-005 | Scaffold Go API service (config, structured logging, `/healthz`) | 4h | W1-001, W1-002 | API boots with env config and returns 200 on `/healthz` |
| W1-006 | Implement `POST /v1/jobs/weather`: validate payload, create IDs, publish Kafka job | 6h | W1-004, W1-005 | Returns 202 with `job_id`; Kafka message conforms to contract |
| W1-007 | Write initial Redis status on submit (`queued`, `0%`) | 3h | W1-004, W1-006 | Key `job:{job_id}:status` created with canonical fields and valid values |
| W1-008 | Scaffold worker Kafka consumer group and weather job handler wiring | 5h | W1-002, W1-004 | Worker consumes jobs and defers offset commit until processing outcome |
| W1-009 | Implement weather processing + normalized output + progress milestones | 6h | W1-008 | Progress transitions follow `0 -> 20 -> 50 -> 80 -> 100`; state updates visible in Redis |
| W1-010 | Persist final result to MongoDB (`job_results`) with idempotency guarantees | 5h | W1-004, W1-009 | Durable document stored once per `job_id`; replays do not duplicate incorrectly |
| W1-011 | RabbitMQ progress request-reply with `correlation_id` + `reply_to` | 6h | W1-004, W1-007, W1-009 | Request/reply contracts match; correlation verified under concurrent requests |
| W1-012 | Implement `GET /v1/jobs/{job_id}/status` backed by RabbitMQ flow | 4h | W1-011 | Correct status lookup, unknown-job handling, timeout handling |
| W1-013 | Basic React UI for weather submit + job status/progress view | 6h | W1-006, W1-012 | User submits weather job and sees state/progress updates end-to-end |
| W1-014 | Stability + failure paths: retry/backoff, structured logs, reconnect/restart behavior | 7h | W1-010, W1-011, W1-012 | Transient failures retried; logs include `job_id`/`trace_id`; restart scenarios pass |
| W1-015 | Quickstart + test/demo checklist and final buffer for defect fixes | 4h | W1-013, W1-014 | Fresh-clone runbook validated; demo flow reproducible |

## Live Task Status

Update this table continuously during Week 1 execution.

| ID | Status | Last Updated | Notes |
|----|--------|--------------|-------|
| W1-001 | Completed | 2026-02-14 | Created Week 1 skeleton (`api`, `worker`, `ui`, `infra/compose`, `contracts`, `docs`) with starter README files. |
| W1-002 | Completed | 2026-02-14 | Compose stack added for Kafka (KRaft), RabbitMQ, Redis, MongoDB with health checks/volumes; Kafka image corrected to `confluentinc/cp-kafka:7.6.1`. |
| W1-003 | Completed | 2026-02-14 | Added smoke scripts (`bootstrap-and-smoke.sh`, `smoke-connectivity.sh`); full connectivity checks passed locally. |
| W1-004 | Not Started | 2026-02-14 | - |
| W1-005 | Not Started | 2026-02-14 | - |
| W1-006 | Not Started | 2026-02-14 | - |
| W1-007 | Not Started | 2026-02-14 | - |
| W1-008 | Not Started | 2026-02-14 | - |
| W1-009 | Not Started | 2026-02-14 | - |
| W1-010 | Not Started | 2026-02-14 | - |
| W1-011 | Not Started | 2026-02-14 | - |
| W1-012 | Not Started | 2026-02-14 | - |
| W1-013 | Not Started | 2026-02-14 | - |
| W1-014 | Not Started | 2026-02-14 | - |
| W1-015 | Not Started | 2026-02-14 | - |

## Implementation Order

1. Foundation first: skeleton + Compose + connectivity (`W1-001` to `W1-003`).
2. Lock contracts before service logic (`W1-004`).
3. Build submit path (`W1-005` to `W1-007`).
4. Build worker execution + persistence (`W1-008` to `W1-010`).
5. Build progress request-reply (`W1-011`, `W1-012`).
6. Build UI (`W1-013`).
7. Harden and demo-ready (`W1-014`, `W1-015`).

## Day-by-Day Plan

### Day 1 - Foundation
- W1-001, W1-002, start W1-003.
- Validate basic service health and network routing.

### Day 2 - API Submission Path
- W1-004, W1-005, W1-006, W1-007.
- Confirm submit -> Kafka enqueue -> initial Redis status.

### Day 3 - Worker + Job Processing
- W1-008, W1-009, W1-010.
- Confirm progress milestones and final Mongo persistence.

### Day 4 - RabbitMQ Progress Request-Reply
- W1-011, W1-012.
- Validate correlation IDs and concurrent progress checks.

### Day 5 - Basic UI
- W1-013.
- Submit and status view working end-to-end for weather jobs.

### Day 6 - Stability + Failure Paths
- W1-014.
- Add retry/backoff, structured correlation logs, and restart/reconnect checks.

### Day 7 - Buffer + Demo Readiness
- W1-015.
- Fix integration defects and run clean-start demo rehearsal.

## Contract Definitions (Canonical Baselines + Week 1 Conventions)

### 1) Kafka Job Message

Topic: `jobs.weather.v1`  
Key: `job_id`

Headers:
- `content-type: application/json`
- `schema-version: 1.0`

Message JSON:

```json
{
  "schema_version": "1.0",
  "job_id": "6aab8fca-7059-40c4-97d4-53f55fd5bf67",
  "job_type": "weather",
  "submitted_at": "2026-02-14T15:12:04Z",
  "trace_id": "5be4d82b-9c6f-4372-81d2-c0dc5a95a7a3",
  "payload": {
    "city": "Austin",
    "country_code": "US",
    "units": "metric"
  }
}
```

Required by canonical spec:
- `job_id`, `job_type`, `submitted_at`, `payload`, `trace_id`

Week 1 validation:
- `job_id`, `trace_id`: UUID v4
- `job_type`: `weather`
- `payload.units`: `metric|imperial`

### 2) Redis Status Schema

Key pattern:
- `job:{job_id}:status`

Canonical required fields:
- `state` (`queued|running|completed|failed`)
- `progress_percent` (`0..100`)
- `updated_at` (RFC3339 UTC)
- `message` (string)

Week 1 optional fields:
- `job_id`
- `trace_id`
- `error_code`
- `error_message`

Recommended TTL:
- 24h on `job:{job_id}:status`

### 3) MongoDB Result Schema

Database: `dtq`  
Collection: `job_results`

Document JSON:

```json
{
  "_id": { "$oid": "67b0f014a9af5f8f0f0cb67d" },
  "schema_version": "1.0",
  "job_id": "6aab8fca-7059-40c4-97d4-53f55fd5bf67",
  "job_type": "weather",
  "input": {
    "city": "Austin",
    "country_code": "US",
    "units": "metric"
  },
  "output": {
    "temperature": 20.4,
    "condition": "Cloudy",
    "humidity_pct": 61,
    "wind_kph": 13.2
  },
  "final_state": "completed",
  "started_at": "2026-02-14T15:12:05Z",
  "completed_at": "2026-02-14T15:12:08Z",
  "error": null,
  "trace_id": "5be4d82b-9c6f-4372-81d2-c0dc5a95a7a3"
}
```

Required by canonical spec:
- `job_id`, `job_type`, `input`, `output`, `final_state`, `started_at`, `completed_at`, `error`

Indexes:
- Unique `{ "job_id": 1 }`
- Optional `{ "completed_at": -1 }`

### 4) RabbitMQ Progress Request-Reply

Request queue:
- `progress.check.request.v1`

Reply queue:
- Per-requester exclusive queue via `reply_to`

AMQP properties:
- `correlation_id` (required)
- `reply_to` (required)
- `content_type=application/json`

Request body:

```json
{
  "job_id": "6aab8fca-7059-40c4-97d4-53f55fd5bf67",
  "request_id": "f2ce7230-d853-4a5f-ab27-bf20a4f5e273",
  "requested_at": "2026-02-14T15:12:05Z"
}
```

Reply body:

```json
{
  "job_id": "6aab8fca-7059-40c4-97d4-53f55fd5bf67",
  "state": "running",
  "progress_percent": 50,
  "message": "calling weather provider",
  "timestamp": "2026-02-14T15:12:06Z"
}
```

Correlation rule:
- API sets `correlation_id = request_id` on request.
- Worker echoes the same `correlation_id` on reply.

## Week 1 Acceptance Checklist

- Weather job runs end-to-end from UI without manual data store edits.
- State transition correctness: `queued -> running -> completed/failed`.
- Progress query returns accurate live state by `job_id`.
- Redis has current state/progress; MongoDB has durable final result.
- Stack starts from fresh clone and demo path runs in under 10 minutes.

## Week 1 Test Checklist

- [ ] Fresh clone setup works with documented steps only.
- [ ] `docker compose up -d` starts Kafka (KRaft), RabbitMQ, Redis, MongoDB healthy.
- [ ] Connectivity smoke checks pass for all core services.
- [ ] `POST /v1/jobs/weather` returns `202` with valid `job_id`.
- [ ] Kafka message includes canonical required fields and valid payload.
- [ ] Redis key `job:{job_id}:status` exists with canonical fields.
- [ ] Worker applies progress milestones exactly: `0 -> 20 -> 50 -> 80 -> 100`.
- [ ] States transition correctly: `queued -> running -> completed` (or `failed`).
- [ ] Mongo `job_results` contains durable final record with canonical required fields.
- [ ] Replay/duplicate delivery does not create duplicate result documents.
- [ ] RabbitMQ progress requests include `request_id` and AMQP `correlation_id`.
- [ ] Reply returns to `reply_to` queue with matching `correlation_id`.
- [ ] Concurrent progress requests by `job_id` return correct responses.
- [ ] API status endpoint handles unknown job and timeout paths safely.
- [ ] UI submit + status flow works without manual Redis/Mongo intervention.
- [ ] Worker retry/backoff handles transient weather API failures.
- [ ] Service restart/reconnect behavior recovers and continues processing.

## Week 1 Demo Checklist

- [ ] Start from clean environment (`docker compose down -v`, then up).
- [ ] Show service health for Kafka, RabbitMQ, Redis, MongoDB.
- [ ] Submit weather job from UI and capture returned `job_id`.
- [ ] Show Kafka message keyed by `job_id`.
- [ ] Show Redis status key updates across milestones and states.
- [ ] Show worker logs with `job_id` and `trace_id` correlation.
- [ ] Show final Mongo `job_results` document.
- [ ] Show RabbitMQ request-reply correlation behavior live.
- [ ] Demonstrate one failure-path run (`failed` + error details).
- [ ] Confirm fresh-clone quickstart and known limitations.

## Reporting Format for Thread Updates

- Scope targeted
- What changed
- Acceptance criteria status
- Risks/blockers
- Next smallest task

## Session Log (Append-Only)

- 2026-02-14: Consolidated Week 1 planning + status into this single source-of-truth file.
- 2026-02-14: Completed W1-001 by creating canonical Week 1 repo/service skeleton and starter docs.
- 2026-02-14: Completed W1-002 by implementing Compose infra with health checks and persistent volumes; replaced unavailable Kafka image tag with a stable one.
- 2026-02-14: Completed W1-003 by adding one-command bootstrap + protocol connectivity checks; user run succeeded with all services healthy.

## Handoff Snapshot

- Week status: In progress
- Completed tasks: W1-001, W1-002, W1-003
- In-progress tasks: None
- Blockers: None
- Next task: W1-004 define canonical Week 1 contracts under `contracts/`
