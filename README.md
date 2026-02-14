# Distributed-Task-Queue

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor real-time progress.

## Test Infra Setup Today (Day 1)
Prerequisite: Docker Desktop (or Docker daemon) must be running.

Optional check:
```bash
docker info > /dev/null && echo "Docker is running"
```

### 1) Start + Validate (one command)
```bash
bash infra/compose/scripts/bootstrap-and-smoke.sh
```

This command will:
- create `infra/compose/.env` from `.env.example` if missing
- start Kafka (KRaft), RabbitMQ, Redis, MongoDB
- run connectivity checks across all four services

### 2) Re-run validation only (if services are already up)
```bash
bash infra/compose/scripts/smoke-connectivity.sh
```

### 3) Check service status
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env ps
```

### 4) Teardown at end of day
Stop containers, keep persisted data:
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down
```

Stop containers and delete persisted volumes/data:
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down -v
```

## Week 1 Layout
- `api/` - temporary Go REST API (submit + status)
- `worker/` - Go Kafka consumer and progress responder
- `ui/` - basic React submit/status interface
- `infra/compose/` - local Kafka/RabbitMQ/Redis/Mongo stack
- `contracts/` - message/data contracts
- `docs/` - canonical spec and execution plans
