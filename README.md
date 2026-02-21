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

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor live progress.

## Current Runbook (Completed Through Week 4 Day 20)
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
