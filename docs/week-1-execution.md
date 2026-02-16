# Week 1 Execution Brief (Single Source of Truth)

Last updated: 2026-02-16  
Canonical product/timeline reference: `docs/project-spec.md` (Week 1 section + acceptance criteria)

This file is the only Week 1 execution and status artifact. It supersedes `docs/week1-execution-plan.md`.

## Objective

Deliver a fully working local end-to-end flow using Docker Compose:
- Submit job (weather profile used for initial testing)
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
- Go worker consuming Kafka and running jobs (weather profile first)
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
| W1-006 | Implement `POST /v1/jobs` (generic submit): validate payload, create IDs, publish Kafka job | 6h | W1-004, W1-005 | Returns 202 with `job_id`; Kafka message conforms to contract |
| W1-007 | Write initial Redis status on submit (`queued`, `0%`) | 3h | W1-004, W1-006 | Key `job:{job_id}:status` created with canonical fields and valid values |
| W1-008 | Scaffold worker Kafka consumer group and job handler wiring (weather profile first) | 5h | W1-002, W1-004 | Worker consumes jobs and defers offset commit until processing outcome |
| W1-009 | Implement initial processing (weather profile) + normalized output + progress milestones | 6h | W1-008 | Progress transitions follow `0 -> 20 -> 50 -> 80 -> 100`; state updates visible in Redis |
| W1-010 | Persist final result to MongoDB (`job_results`) with idempotency guarantees | 5h | W1-004, W1-009 | Durable document stored once per `job_id`; replays do not duplicate incorrectly |
| W1-011 | RabbitMQ progress request-reply with `correlation_id` + `reply_to` | 6h | W1-004, W1-007, W1-009 | Request/reply contracts match; correlation verified under concurrent requests |
| W1-012 | Implement `GET /v1/jobs/{job_id}/status` backed by RabbitMQ flow | 4h | W1-011 | Correct status lookup, unknown-job handling, timeout handling |
| W1-013 | Basic React UI for generic submit + job status/progress view (weather test profile) | 6h | W1-006, W1-012 | User submits a job (weather profile) and sees state/progress updates end-to-end |
| W1-014 | Stability + failure paths: retry/backoff, structured logs, reconnect/restart behavior | 7h | W1-010, W1-011, W1-012 | Transient failures retried; logs include `job_id`/`trace_id`; restart scenarios pass |
| W1-015 | Quickstart + test/demo checklist and final buffer for defect fixes | 4h | W1-013, W1-014 | Fresh-clone runbook validated; demo flow reproducible |

## Live Task Status

Update this table continuously during Week 1 execution.

| ID | Status | Last Updated | Notes |
|----|--------|--------------|-------|
| W1-001 | Completed | 2026-02-14 | Created Week 1 skeleton (`api`, `worker`, `ui`, `infra/compose`, `contracts`, `docs`) with starter README files. |
| W1-002 | Completed | 2026-02-14 | Compose stack added for Kafka (KRaft), RabbitMQ, Redis, MongoDB with health checks/volumes; Kafka image corrected to `confluentinc/cp-kafka:7.6.1`. |
| W1-003 | Completed | 2026-02-14 | Added smoke scripts (`bootstrap-and-smoke.sh`, `smoke-connectivity.sh`); full connectivity checks passed locally. |
| W1-004 | Completed | 2026-02-14 | Added canonical Week 1 contract schemas + examples under `contracts/kafka`, `contracts/redis`, `contracts/mongo`, and `contracts/rabbitmq`. |
| W1-005 | Completed | 2026-02-14 | Scaffolded Go API with env config, structured logging, and dependency-aware `GET /healthz`. |
| W1-006 | Completed | 2026-02-14 | Implemented `POST /v1/jobs` generic submit with payload validation, UUID IDs, and Kafka publish to per-job-type topics. |
| W1-007 | Completed | 2026-02-14 | API now writes initial Redis status key `job:{job_id}:status` with `queued` + `progress_percent=0` and TTL before enqueue. |
| W1-008 | Completed | 2026-02-15 | Worker scaffold added with Kafka consumer-group loop, job-type handler wiring, and manual commit control (`FetchMessage` + `CommitMessages`) after processing outcome. |
| W1-009 | Completed | 2026-02-15 | Worker now executes weather jobs with normalized output and Redis progress milestones (`running:20 -> running:50 -> running:80 -> completed:100`). |
| W1-010 | Completed | 2026-02-15 | Worker persists final results to MongoDB `job_results` using idempotent upsert (`$setOnInsert`) and unique `job_id` index. |
| W1-011 | Completed | 2026-02-16 | Worker now serves RabbitMQ progress request-reply on `progress.check.request.v1`, requiring `correlation_id` + `reply_to` and echoing correlation on replies. |
| W1-012 | Completed | 2026-02-16 | API now exposes `GET /v1/jobs/{job_id}/status` backed by RabbitMQ request-reply with timeout + unknown-job handling. |
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
- Submit and status view working end-to-end for generic jobs (weather profile tested first).

### Day 6 - Stability + Failure Paths
- W1-014.
- Add retry/backoff, structured correlation logs, and restart/reconnect checks.

### Day 7 - Buffer + Demo Readiness
- W1-015.
- Fix integration defects and run clean-start demo rehearsal.

## Contract Definitions (Canonical Baselines + Week 1 Conventions)

### 1) Kafka Job Message

Topic: `jobs.{job_type}.v1` (default from `KAFKA_TOPIC_TEMPLATE=jobs.%s.v1`)  
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
- `job_type`: one of `weather|quote|exchange_rate|github_user`
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
  "message": "calling external provider",
  "timestamp": "2026-02-14T15:12:06Z"
}
```

Correlation rule:
- API sets `correlation_id = request_id` on request.
- Worker echoes the same `correlation_id` on reply.

## Week 1 Acceptance Checklist

- Job flow runs end-to-end from UI without manual data store edits (validated with weather profile).
- State transition correctness: `queued -> running -> completed/failed`.
- Progress query returns accurate live state by `job_id`.
- Redis has current state/progress; MongoDB has durable final result.
- Stack starts from fresh clone and demo path runs in under 10 minutes.

## Week 1 Test Checklist

- [ ] Fresh clone setup works with documented steps only.
- [ ] `bash scripts/run-current-e2e.sh` passes from a clean local environment.
- [ ] `docker compose up -d` starts Kafka (KRaft), RabbitMQ, Redis, MongoDB healthy.
- [ ] Connectivity smoke checks pass for all core services.
- [ ] `POST /v1/jobs` returns `202` with valid `job_id`.
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
- [ ] Worker retry/backoff handles transient provider API failures (weather profile validated first).
- [ ] Service restart/reconnect behavior recovers and continues processing.

## Week 1 Demo Checklist

- [ ] Start from clean environment (`docker compose down -v`, then up).
- [ ] Show service health for Kafka, RabbitMQ, Redis, MongoDB.
- [ ] Submit one test job from UI (weather profile) and capture returned `job_id`.
- [ ] Show Kafka message keyed by `job_id`.
- [ ] Show Redis status key updates across milestones and states.
- [ ] Show worker logs with `job_id` and `trace_id` correlation.
- [ ] Show final Mongo `job_results` document.
- [ ] Show RabbitMQ request-reply correlation behavior live.
- [ ] Demonstrate one failure-path run (`failed` + error details).
- [ ] Confirm fresh-clone quickstart and known limitations.

## Reporting Format for Thread Updates

Use the canonical format in `docs/agent/handoff-template.md` and avoid repeating that structure here.

## Session Log (Append-Only)

- 2026-02-14: Consolidated Week 1 planning + status into this single source-of-truth file.
- 2026-02-14: Completed W1-001 by creating canonical Week 1 repo/service skeleton and starter docs.
- 2026-02-14: Completed W1-002 by implementing Compose infra with health checks and persistent volumes; replaced unavailable Kafka image tag with a stable one.
- 2026-02-14: Completed W1-003 by adding one-command bootstrap + protocol connectivity checks; user run succeeded with all services healthy.
- 2026-02-14: Completed W1-004 by adding contract schema/example files for Kafka, Redis, MongoDB, and RabbitMQ under `contracts/`.
- 2026-02-14: Completed W1-005/W1-006/W1-007 by implementing Go API scaffold + generic `POST /v1/jobs` enqueue flow and initial Redis queued status writes.
- 2026-02-14: Removed `POST /v1/jobs/weather` alias to keep a single generic submission path from project start.
- 2026-02-14: Refactored submission path to be job-type extensible (`weather`, `quote`, `exchange_rate`, `github_user`) with topic template routing and per-type payload validators.
- 2026-02-14: Local environment lacks Go toolchain (`go`/`gofmt` missing), so compile/test validation is pending on a machine with Go installed.
- 2026-02-14: Added Day 2 API validation commands to `README.md` (compile/test/run + Redis/Kafka verification path).
- 2026-02-14: Consolidated `README.md` into a single cumulative runbook (completed-through-Day-2) and removed day-split instructions.
- 2026-02-14: Added explicit agent policy/workflow rule: README must always keep only current Day N cumulative setup/test instructions and remove Day (N-1) sections.
- 2026-02-14: Added explicit policy/workflow rule: execute one task ID at a time (no auto-batching all tasks for a day) unless user explicitly requests batching.
- 2026-02-14: Updated `api/main.go` with function-level comments + logging coverage and added policy rule requiring comments/logging for new or modified functions.
- 2026-02-14: Generalized remaining docs/readmes to job-type-first wording; kept weather as initial test profile example only.
- 2026-02-14: Updated README Kafka verification command to use `--from-beginning` so manual checks can validate previously submitted messages without timing windows.
- 2026-02-15: Closed Day 2 checkpoint after user-validated infra/API submit flow (`/healthz`, `POST /v1/jobs`, Redis status, Kafka read with `--from-beginning`); next implementation task remains `W1-008`.
- 2026-02-15: Completed W1-008 by implementing `worker` Kafka consumer-group scaffold with job-type handler registry (weather-first wiring), graceful shutdown loop, and deferred offset commits based on processing outcome.
- 2026-02-15: Added worker unit tests for commit semantics (commit on success, skip commit on handler failure, commit dropped invalid messages); sandbox cannot fetch Go modules, so local execution is pending on developer machine.
- 2026-02-15: Completed W1-009 by adding worker execution flow for weather jobs with provider abstraction (`openmeteo` + optional mock fallback), payload normalization, and Redis milestone updates (`20/50/80/100`).
- 2026-02-15: Completed W1-010 by adding MongoDB final-result persistence (`job_results`) with unique `job_id` index and idempotent `Upsert` behavior to prevent duplicate durable records on replays.
- 2026-02-15: Updated root `README.md` to Day-3 cumulative runbook (infra + API + worker + Redis/Mongo verification + teardown), replacing Day-2-only operational scope.
- 2026-02-16: Completed W1-011 by adding worker RabbitMQ progress responder with strict request validation, Redis-backed status lookup, and AMQP correlation/reply semantics.
- 2026-02-16: Completed W1-012 by implementing API `GET /v1/jobs/{job_id}/status` over RabbitMQ request-reply with UUID path validation, timeout handling, and `not_found` mapping.
- 2026-02-16: Updated root README/API/worker docs to Day-4 cumulative runbook and environment variables for RabbitMQ progress flow.
- 2026-02-16: Added deterministic current E2E runner (`scripts/run-current-e2e.sh`) that isolates topic/queue per run and performs automatic API/worker startup, validation, and teardown.
- 2026-02-16: Hardened connectivity smoke checks with startup retries for stable script-driven validation during local container startup.
- 2026-02-16: Added workflow policy to keep `scripts/run-current-e2e.sh` updated as Day N advances and keep README aligned to that single canonical command.
- 2026-02-16: Validated `scripts/run-current-e2e.sh` end-to-end locally (submit -> RabbitMQ status -> Redis completed -> Mongo persisted -> Kafka consumed) with automatic teardown.
- 2026-02-16: De-duplicated instruction docs by keeping detailed execution rules in `docs/agent/workflow.md` and replacing repeated reporting-format bullets in this file with a pointer to `docs/agent/handoff-template.md`.
- 2026-02-16: Sandbox cannot resolve `proxy.golang.org`, so `go mod tidy`/`go test` for updated API/worker modules must be executed on the local developer machine.

## Handoff Snapshot

- Week status: In progress
- Completed tasks: W1-001, W1-002, W1-003, W1-004, W1-005, W1-006, W1-007, W1-008, W1-009, W1-010, W1-011, W1-012
- In-progress tasks: None
- Blockers: None
- Day checkpoint: Day 4 complete
- Next task: W1-013 implement basic React UI submit + status view
