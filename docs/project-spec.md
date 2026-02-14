# Distributed Task Queue - Canonical Project Specification

Last updated: 2026-02-14
Owner: Project team
Purpose: Single source of truth for architecture, timeline, and acceptance criteria across all implementation threads and sub-agents.

## 1) Project Overview

Build a distributed task queue where users submit jobs via a web UI, workers process those jobs, and users monitor progress in near real time.

### Initial Job Types
- Weather API calls
- Public quote API calls
- Exchange rate API calls
- GitHub user info API calls

## 2) High-Level Architecture

```text
┌──────────────┐   GraphQL   ┌──────────────┐         ┌──────────────┐
│   React UI   │────────────▶│    Go API    │────────▶│    Kafka     │
└──────────────┘             └──────┬───────┘         └──────┬───────┘
                                    │                        │
                                    │ gRPC                   ▼
                                    ▼                 ┌──────────────┐
                             ┌──────────────┐         │  Go Workers  │
                             │  Worker Svc  │◀───────▶│              │
                             └──────────────┘         └──────┬───────┘
                                    ▲                        │
                                    │ RabbitMQ               │
                                    │ (progress req/reply)   ▼
                             ┌──────────────┐         ┌──────────────┐
                             │    Redis     │◀────────│   MongoDB    │
                             └──────────────┘         └──────────────┘

Monitoring: Prometheus -> Grafana
CI/CD: Jenkins (on Azure VM)
Infrastructure: Ansible -> Azure AKS
```

## 3) Technology Roles

| Technology | Role |
|------------|------|
| Kafka | Job submission queue (fire-and-forget) |
| RabbitMQ | Live progress request-reply |
| Redis | Fast cache for current job status/progress |
| MongoDB | Durable persistence for job metadata/results |
| Go | API and worker services |
| GraphQL | UI-facing API (queries, mutations, subscriptions) |
| gRPC | Internal typed service communication |
| React | Job submission and monitoring UI |
| Docker | Service containerization |
| Kubernetes (AKS) | Cloud orchestration |
| Ansible | Azure provisioning (AKS, ACR, Jenkins VM) |
| Jenkins | Build/test/push/deploy pipeline |
| Prometheus | Metrics scraping/collection |
| Grafana | Dashboards and alerting |

## 4) Core Processing Flow

1. User submits job from React UI (GraphQL mutation; Week 1 uses temporary REST).
2. Go API publishes job message to Kafka.
3. Go worker consumes from Kafka and calls external API.
4. Worker updates progress/status in Redis.
5. Worker persists final result and metadata in MongoDB.
6. User requests live progress via RabbitMQ request-reply.
7. React UI receives updates through GraphQL subscriptions (from Week 2 onward).

## 5) Week-by-Week Delivery Plan

## Week 1 - Core Functionality (Local Docker Compose)

Environment: Local (`docker-compose`)

### Goals
- Bring up Kafka (KRaft), RabbitMQ, Redis, MongoDB with Docker Compose.
- Implement temporary Go REST API for job submission/progress access.
- Implement Go worker consuming Kafka and processing at least weather jobs.
- Add RabbitMQ request-reply for live progress checks.
- Build basic React UI for submit + status view.

### Detailed Task Plan

1. Day 1 - Foundation
- Define repo/service skeleton (`api`, `worker`, `ui`, `infra/compose`).
- Create compose stack and health checks.
- Validate local inter-service connectivity.

2. Day 2 - API Submission Path
- Build REST endpoint for weather job submission.
- Validate payload and generate `job_id`.
- Publish job to Kafka and set initial Redis status (`queued`).

3. Day 3 - Worker + Job Processing
- Consume jobs from Kafka.
- Execute weather API call and normalize response.
- Update progress milestones (`0 -> 20 -> 50 -> 80 -> 100`).
- Persist final result to MongoDB.

4. Day 4 - RabbitMQ Progress Request-Reply
- Add progress request queue and reply handling.
- Use correlation IDs and `reply_to`.
- Expose API route for progress lookup backed by RabbitMQ flow.

5. Day 5 - Basic UI
- Add weather job form and submission flow.
- Add status/progress view by `job_id`.
- Render state changes (`queued/running/completed/failed`).

6. Day 6 - Stability + Failure Paths
- Add retry/backoff for transient external API errors.
- Add structured logs with `job_id` correlation.
- Validate reconnect/restart behavior for key services.

7. Day 7 - Buffer + Demo Readiness
- Fix integration defects.
- Prepare quickstart and demo checklist.
- Re-run end-to-end validation from clean startup.

### Week 1 Acceptance Criteria
- End-to-end weather job flow works via UI with no manual DB/cache edits.
- Status transitions are visible and correct.
- RabbitMQ progress query by `job_id` is correct under concurrent requests.
- Redis stores current status/progress; MongoDB stores durable final result.
- Stack can be launched from fresh clone with documented steps.

## Week 2 - GraphQL, gRPC, Real-Time (Local)

Environment: Local (`docker-compose`)

### Goals
- Replace temporary REST with GraphQL API.
- Implement GraphQL queries, mutations, subscriptions.
- Add gRPC between API and worker service.
- Use Apollo Client + WebSocket subscriptions in React.
- Deliver live progress updates without polling.

### Acceptance Criteria
- Job submission works via GraphQL mutation.
- REST paths are removed or disabled.
- Live progress appears via GraphQL subscriptions.
- API-worker communication uses defined gRPC contracts.
- Week 1 behavior remains functional after migration.

## Week 3 - Azure Deployment (Ansible, AKS, Jenkins)

Environment: Azure

### Goals
- Provision AKS, ACR, and Jenkins VM through Ansible.
- Create Kubernetes manifests for all services.
- Build/push images to ACR and deploy to AKS.
- Implement Jenkins CI/CD pipeline: build -> test -> push -> deploy.
- Expose application publicly.

### Acceptance Criteria
- Infra provisioning works from a clean Azure setup.
- CI/CD pipeline deploys without manual deployment steps.
- Cloud deployment supports successful end-to-end job processing.
- Public endpoint is reachable and usable.

## Week 4 - Monitoring, Hardening, Polish (Azure)

Environment: Azure

### Goals
- Deploy Prometheus and Grafana to AKS.
- Instrument Go services (`/metrics`).
- Add worker retries, error handling, graceful shutdown.
- Finalize docs/README/runbook.

### Acceptance Criteria
- Dashboards show throughput, error rate, duration, worker health.
- Metrics and alerts support operational troubleshooting.
- In-flight jobs are handled safely during worker shutdown.
- Project docs are complete enough for new contributors to run and operate.

## 6) Design Decisions

- Kafka handles async fire-and-forget submission at scale.
- RabbitMQ is used for request-reply progress queries requiring correlation/ack semantics.
- GraphQL chosen over REST for flexible client queries and subscriptions.
- gRPC chosen for efficient typed internal communication.
- Redis is source of current transient status; MongoDB is source of durable results.
- Local Kubernetes is intentionally skipped; progression is Compose -> AKS.
- Jenkins is hosted in Azure only (no local Jenkins requirement).

## 7) Data and Contract Baselines (Week 1-first)

These are baseline contracts to establish before implementation:

1. Kafka job message schema
- Required: `job_id`, `job_type`, `submitted_at`, `payload`, `trace_id`

2. Redis status schema
- Key pattern: `job:{job_id}:status`
- Fields: `state`, `progress_percent`, `updated_at`, `message`

3. MongoDB result schema
- Collection: `job_results`
- Required: `job_id`, `job_type`, `input`, `output`, `final_state`, `started_at`, `completed_at`, `error`

4. RabbitMQ progress request-reply
- Request: `job_id`, `request_id`, `requested_at`
- Reply: `job_id`, `state`, `progress_percent`, `message`, `timestamp`
- Correlation: use AMQP `correlation_id`; return to `reply_to`

## 8) External API Dependencies

- OpenWeatherMap
- Public quote API (final provider TBD during implementation)
- Exchange rate API (final provider TBD during implementation)
- GitHub API (user info)

## 9) Non-Goals (Current Plan)

- No local Kubernetes deployment stage.
- No advanced multi-region HA design in initial 4-week scope.
- No production-grade auth/tenancy hardening specified yet.

## 10) Risks and Assumptions

### Risks
- Running both Kafka and RabbitMQ increases operational complexity.
- GraphQL subscriptions over WebSockets may require ingress/session tuning on AKS.
- External API rate limits/failures can impact worker reliability.
- Azure IAM/provisioning friction can delay Week 3 if not validated early.

### Assumptions
- Team has Azure subscription access with rights for AKS, VM, ACR.
- Local developer machines can run full Docker Compose stack.
- Secrets management strategy (for APIs and Azure creds) is defined during implementation.

## 11) Agent Handoff Notes

- This file is the canonical reference for milestones, scope, and acceptance criteria.
- Implementation threads should link back to this file and avoid drifting requirements.
- Any scope/timeline changes should be appended below in the change log.

## 12) Change Log

- 2026-02-14: Created canonical specification from planning discussion and added detailed Week 1 execution/acceptance criteria.
