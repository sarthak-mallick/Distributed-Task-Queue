# Distributed-Task-Queue

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor live progress.

## Current Runbook (Completed Through Week 4 Day 17)
This runbook reflects the latest completed implementation slice and supersedes prior flow details.

Prerequisites:
- Docker Desktop (or Docker daemon) running
- Go toolchain installed (Go 1.23+)
- Node.js + npm installed (Node 20+ recommended)
- `kubectl` installed (required for Week 4 Day 17 AKS/monitoring validation checks)
- `ansible-playbook` installed (optional: if unavailable, the inherited Week 3 Ansible preflight check is skipped with a log note)

### 1) Start Infra + Verify Connectivity + Canonical Kafka Topics
```bash
bash infra/compose/scripts/bootstrap-and-smoke.sh
```

### 2) Run Worker (Terminal 1)
```bash
cd worker
go run .
```

### 3) Run API (Terminal 2)
```bash
cd api
go run .
```

### 4) Run UI (Terminal 3)
```bash
cd ui
npm install
npm run dev
```

Open `http://localhost:5173`.

### 5) Validate Through UI (GraphQL + Subscriptions)

1. Submit a `weather` job from the UI.
2. Confirm submit response contains `jobId` and `queued` state.
3. Confirm status tracker receives live progress updates over WebSocket subscription.
4. Confirm final terminal state is `completed` with `100%` progress.

### 6) Full Automated Validation (Recommended)
Use this to run the full current validation flow and purge volumes at teardown:
```bash
bash scripts/run-current-e2e.sh --with-ui-checks --purge
```

Current validation coverage in `run-current-e2e.sh`:
- Week 4 Day 17 scaffold checks:
  - Jenkinsfile stage + shared image contract verification (`${ACR_LOGIN_SERVER}/dtq-<component>:${IMAGE_TAG}`)
  - AKS deploy helper verification + dry run (`infra/aks/scripts/deploy-release.sh`)
  - Week 4 execution artifact verification (`docs/week-4-execution.md`)
  - AKS monitoring manifest render checks (`infra/aks/monitoring`)
  - Containerization artifact checks (`api/worker/ui` Dockerfiles + `ui/nginx.conf`)
  - Local Docker image build checks for `api`, `worker`, and `ui` (can be skipped for faster runs)
  - `kubectl kustomize infra/aks/base` render validation
  - AKS manifest contract checks (`dtq-api-config`, `dtq-worker-config`, `dtq-runtime-secrets`, `dtq-worker-grpc`, `dtq-worker-metrics`)
  - API/worker deployment env-wiring validation (including `WORKER_METRICS_ADDR`)
  - Ansible preflight playbook check (when `ansible-playbook` is installed)
- Week 2 runtime checks:
  - UI build checks (optional via `--with-ui-checks`)
  - API/worker unit tests
  - GraphQL mutation/query/subscription end-to-end flow
  - API/worker `/metrics` endpoint and counter validation
  - Redis/Mongo/Kafka data validation

Optional flags:
- run backend-only checks: `bash scripts/run-current-e2e.sh`
- keep containers running: `bash scripts/run-current-e2e.sh --keep-infra`
- skip Go unit tests: `bash scripts/run-current-e2e.sh --skip-unit-tests`
- skip local image builds: `bash scripts/run-current-e2e.sh --skip-image-build-checks`
- skip Week 4 Day 17 scaffold checks: `bash scripts/run-current-e2e.sh --skip-week4-checks` (or `--skip-week3-checks` alias)
- include frontend checks (`npm install/ci` + `npm run build`): `bash scripts/run-current-e2e.sh --with-ui-checks`
- purge compose volumes at teardown: `bash scripts/run-current-e2e.sh --purge`

### 7) Teardown
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down
```

To remove persisted data volumes too:
```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down -v
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
