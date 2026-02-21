# Distributed-Task-Queue

## Architecture Diagram

```mermaid
flowchart TB
    subgraph Client["Client"]
        UI["React UI"]
    end

    subgraph App["Application Services (Go)"]
        API["Go API (GraphQL + gRPC)"]
        WSVC["Worker Service"]
        WKR["Go Workers"]
    end

    subgraph Messaging["Messaging Layer"]
        KAFKA["Kafka (Job Queue)"]
        RMQ["RabbitMQ (Progress Req/Reply)"]
    end

    subgraph Data["Data Layer"]
        REDIS["Redis (Status Cache)"]
        MONGO["MongoDB (Durable Results)"]
    end

    subgraph Obs["Observability"]
        PROM["Prometheus"]
        GRAF["Grafana"]
    end

    subgraph Cloud["Azure Delivery"]
        ANS["Ansible (Provisioning)"]
        JENK["Jenkins (CI/CD)"]
        ACR["Azure Container Registry"]
        AKS["Azure AKS"]
    end

    subgraph Ext["External APIs"]
        WEATHER["Weather API"]
        QUOTE["Quote API"]
        FX["Exchange Rate API"]
        GITHUB["GitHub API"]
    end

    UI -->|"GraphQL queries/mutations/subscriptions"| API
    API -->|"Internal calls"| WSVC
    API -->|"Publish jobs"| KAFKA
    KAFKA -->|"Consume jobs"| WKR
    WSVC <--> WKR

    WKR -->|"Call provider"| WEATHER
    WKR -->|"Call provider"| QUOTE
    WKR -->|"Call provider"| FX
    WKR -->|"Call provider"| GITHUB

    API -->|"Progress request"| RMQ
    RMQ -->|"Progress reply (correlation_id)"| API
    WKR -->|"Status/progress"| REDIS
    WKR -->|"Final results"| MONGO

    PROM -->|"Scrape /metrics"| API
    PROM -->|"Scrape /metrics"| WKR
    GRAF -->|"Dashboards/alerts"| PROM

    ANS -->|"Provision infra"| AKS
    ANS -->|"Provision CI host"| JENK
    JENK -->|"Build/test/push images"| ACR
    JENK -->|"Deploy manifests"| AKS
    AKS -->|"Runs services"| API
    AKS -->|"Runs services"| WKR
```

### Architecture Summary

- The React UI is the user entry point for job submission and live progress monitoring.
- The Go API exposes the GraphQL surface and orchestrates job lifecycle operations.
- Kafka is the primary asynchronous queue used to decouple job submission from processing.
- Go workers consume Kafka jobs, execute external API calls, and drive progress milestones.
- Redis stores hot job state (`queued`, `running`, `completed/failed`) for fast status reads.
- MongoDB stores durable final outputs and metadata for each job execution.
- RabbitMQ handles request-reply style progress lookups where correlation and acknowledgments matter.
- Prometheus and Grafana provide metrics, dashboards, and alerting for service and queue health.
- Azure delivery uses Ansible + Jenkins + ACR + AKS for provisioning, CI/CD, and runtime hosting.

## Workflow Diagram

```mermaid
sequenceDiagram
    actor User
    participant UI as React UI
    participant API as Go API
    participant K as Kafka
    participant W as Go Worker
    participant R as Redis
    participant M as MongoDB
    participant Q as RabbitMQ

    User->>UI: Submit job
    UI->>API: GraphQL mutation (submitJob)
    API->>K: Publish job event
    API->>R: Set status = queued (0%)
    API-->>UI: Return job_id

    K->>W: Deliver job
    W->>R: Update status = running (20/50/80%)
    W->>W: Call external API (weather/quote/fx/github)
    W->>M: Persist final result
    W->>R: Update status = completed (100%)

    UI->>API: GraphQL subscription (job updates)
    API-->>UI: Push status/progress events

    User->>UI: Request current progress
    UI->>API: Query progress by job_id
    API->>Q: Progress request (correlation_id)
    Q-->>API: Progress reply
    API-->>UI: Current status/progress
```

### Workflow Explanation

When a user submits a job from the React UI, the API accepts the request and returns a `job_id` immediately so the UI can start tracking progress.  
The API publishes the job to Kafka, which decouples request handling from worker execution and keeps submission responsive under load.  
Workers consume jobs asynchronously, execute the external provider call, and update Redis with milestone progress states during processing.  
Redis acts as the fast source of truth for current status, so progress checks can be served quickly without scanning durable storage.  
Once processing finishes, the worker persists the final result and metadata in MongoDB for durable retrieval and auditing.  
In parallel, the UI receives live updates through GraphQL subscriptions pushed from the API as job state changes occur.  
If a user explicitly asks for current progress, the API uses RabbitMQ request-reply to fetch correlated status data reliably.  
This split design lets Kafka handle high-throughput job ingestion while RabbitMQ handles interactive, correlation-sensitive progress lookups.

## Runbooks
Runbooks are split by use case to avoid duplicated instructions:
Use the split runbooks:
- Local app run: `docs/runbooks/local-runbook.md`
- Local validation/testing: `docs/runbooks/local-validation-runbook.md`
- Azure/AKS deployment: `docs/runbooks/azure-runbook.md`

Quick validation entrypoint:

```bash
bash scripts/run-current-e2e.sh --with-ui-checks --purge
```

## Current Layout
- `api/` - Go API with GraphQL mutation/query/subscription surface and Kafka enqueue path
- `worker/` - Go worker Kafka consumer + gRPC status/progress service + RabbitMQ responder compatibility
- `ui/` - React Apollo client UI with GraphQL WebSocket subscriptions
- `infra/compose/` - local Kafka/RabbitMQ/Redis/Mongo stack
- `infra/aks/monitoring/` - Week 4 Prometheus/Grafana AKS manifests
- `contracts/` - Week 1 data contracts + Week 2 gRPC contract
- `docs/` - canonical spec and active week execution plans
- `scripts/` - deterministic local test runners
