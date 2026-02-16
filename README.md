# Distributed-Task-Queue

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor real-time progress.

## Current Runbook (Completed Through Day 5)
This runbook reflects the latest completed implementation slice and supersedes prior day-specific steps.

Prerequisites:
- Docker Desktop (or Docker daemon) running
- Go toolchain installed (Go 1.22+)
- Node.js + npm installed (Node 20+ recommended)

### 1) Start Infra + Verify Connectivity
```bash
bash infra/compose/scripts/bootstrap-and-smoke.sh
```

### 2) Run API (Terminal 1)
```bash
cd api
go run .
```

### 3) Run Worker (Terminal 2)
```bash
cd worker
go run .
```

### 4) Run UI (Terminal 3)
```bash
cd ui
npm install
npm run dev
```

Open `http://localhost:5173`.

### 5) Validate Through UI

1. Submit a `weather` job from the UI.
2. Confirm submit response returns `job_id` and accepted state.
3. Confirm status tracker progresses to `completed` with `100%`.
4. Optionally paste any `job_id` into status tracker and check/poll manually.

### 6) Optional Backend Automation Check
Use this to re-run backend end-to-end validation from one command:
```bash
bash scripts/run-current-e2e.sh
```

Optional flags:
- keep containers running: `bash scripts/run-current-e2e.sh --keep-infra`
- skip Go unit tests: `bash scripts/run-current-e2e.sh --skip-unit-tests`
- include frontend checks (`npm install/ci` + `npm run build`): `bash scripts/run-current-e2e.sh --with-ui-checks`
- teardown with volume cleanup: `bash scripts/run-current-e2e.sh --purge`

### 7) Teardown
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down
```

To remove persisted data volumes too:
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
