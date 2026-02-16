# Distributed-Task-Queue

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor real-time progress.

## Current Runbook (Completed Through Day 4)
This runbook reflects the latest completed implementation slice and supersedes prior day-specific steps.

Prerequisites:
- Docker Desktop (or Docker daemon) running
- Go toolchain installed (Go 1.22+)

### 1) Run Full Current Validation (Recommended)
```bash
bash scripts/run-current-e2e.sh
```

What this command does:
- creates `infra/compose/.env` from `.env.example` if missing
- starts Kafka (KRaft), RabbitMQ, Redis, MongoDB
- runs connectivity smoke checks
- runs `go test ./...` for `api` and `worker`
- starts API + worker with isolated Kafka topic and RabbitMQ queue for this run
- submits a weather-profile job and waits for `completed` + `100%`
- validates RabbitMQ status flow, Redis final status, Mongo persistence, Kafka message
- tears everything down automatically by default

### 2) Keep Infra Running After Test (Optional)
```bash
bash scripts/run-current-e2e.sh --keep-infra
```

### 3) Skip Unit Tests During E2E (Optional)
```bash
bash scripts/run-current-e2e.sh --skip-unit-tests
```

### 4) Manual Teardown (Only Needed With `--keep-infra`)
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down
```

To also remove persisted data volumes:
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
- `scripts/` - deterministic local test runners
