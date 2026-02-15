# Distributed-Task-Queue

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor real-time progress.

## Current Runbook (Completed Through Day 2)
This runbook reflects the latest completed implementation slice and supersedes prior day-specific steps.

Prerequisites:
- Docker Desktop (or Docker daemon) running
- Go toolchain installed (Go 1.22+)

Optional check:
```bash
docker info > /dev/null && echo "Docker is running"
```

Optional Go check:
```bash
go version
```

### 1) Start Infra + Validate
```bash
bash infra/compose/scripts/bootstrap-and-smoke.sh
```

This does all Day 1 foundation setup:
- create `infra/compose/.env` from `.env.example` if missing
- start Kafka (KRaft), RabbitMQ, Redis, MongoDB
- run connectivity checks across all four services

### 2) Build + Unit Test API
```bash
cd api
go mod tidy
go test ./...
```

### 3) Run API Service
```bash
cd api
go run .
```

### 4) Validate API Health
```bash
curl http://localhost:8080/healthz
```

### 5) Submit Test Job (Weather Profile)
```bash
curl -s -X POST http://localhost:8080/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "job_type":"weather",
    "payload":{
      "city":"Austin",
      "country_code":"US",
      "units":"metric"
    }
  }'
```

Expected:
- HTTP `202`
- JSON includes `job_id`, `trace_id`, `job_type`, `state: "queued"`

Supported `job_type` values in API submission:
- `weather`
- `quote`
- `exchange_rate`
- `github_user`

### 6) Verify Redis Status + TTL
```bash
JOB_ID=<job_id_from_response>
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env exec -T redis redis-cli HGETALL job:${JOB_ID}:status
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env exec -T redis redis-cli TTL job:${JOB_ID}:status
```

### 7) Verify Kafka Message
For this weather test profile, the topic is `jobs.weather.v1`.  
`--from-beginning` ensures you can read an already-published message (no timing race with submit).
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env exec -T kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic jobs.weather.v1 --from-beginning --max-messages 1
```

### 8) Optional Infra Status Check
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env ps
```

### 9) Teardown
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
