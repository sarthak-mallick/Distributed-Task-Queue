#!/usr/bin/env bash
set -euo pipefail

# run-current-e2e executes deterministic end-to-end validation for the latest Day N flow.
# It isolates test traffic with a unique Kafka topic and RabbitMQ queue per run.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_DIR="${ROOT_DIR}/infra/compose"
COMPOSE_FILE="${COMPOSE_DIR}/docker-compose.yml"
ENV_FILE="${COMPOSE_DIR}/.env"
ENV_EXAMPLE="${COMPOSE_DIR}/.env.example"
API_DIR="${ROOT_DIR}/api"
WORKER_DIR="${ROOT_DIR}/worker"
UI_DIR="${ROOT_DIR}/ui"

KEEP_INFRA=false
SKIP_UNIT_TESTS=false
PURGE_VOLUMES=false
WITH_UI_CHECKS=false

# log prints all script status lines with a stable prefix.
log() {
  printf '[e2e] %s\n' "$1"
}

# usage prints script options and exits.
usage() {
  cat <<'EOF'
Usage: bash scripts/run-current-e2e.sh [options]

Options:
  --keep-infra         Keep Docker Compose services running after the test.
  --skip-unit-tests    Skip `go test ./...` for api and worker.
  --with-ui-checks     Run frontend validation (`npm install/ci` + `npm run build`).
  --purge              Remove compose volumes on teardown (`docker compose down -v`).
  -h, --help           Show this help message.

Behavior:
  1) Starts compose infrastructure and smoke-checks it.
  2) Optionally runs frontend build checks (`--with-ui-checks`).
  3) Runs api/worker unit tests (unless skipped).
  4) Starts API and worker with isolated topic/queue settings.
  5) Submits a weather-profile job and validates status/Redis/Mongo/Kafka.
  6) Tears down app processes and infra by default.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep-infra)
      KEEP_INFRA=true
      shift
      ;;
    --skip-unit-tests)
      SKIP_UNIT_TESTS=true
      shift
      ;;
    --with-ui-checks)
      WITH_UI_CHECKS=true
      shift
      ;;
    --purge)
      PURGE_VOLUMES=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      log "unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

if [[ "${KEEP_INFRA}" == "true" && "${PURGE_VOLUMES}" == "true" ]]; then
  log "invalid option combination: --keep-infra and --purge cannot be used together"
  exit 1
fi

RUN_ID="$(date -u +%Y%m%d%H%M%S)"
TOPIC="jobs.weather.e2e.${RUN_ID}"
PROGRESS_QUEUE="progress.check.request.e2e.${RUN_ID}"
WORKER_GROUP_ID="dtq-worker-e2e-${RUN_ID}"
PROGRESS_CONSUMER_TAG="dtq-worker-progress-e2e-${RUN_ID}"
API_PORT="${API_PORT:-18080}"
API_ADDR=":${API_PORT}"
API_BASE_URL="http://127.0.0.1:${API_PORT}"

LOG_DIR="${ROOT_DIR}/.tmp/e2e/${RUN_ID}"
mkdir -p "${LOG_DIR}"
API_LOG="${LOG_DIR}/api.log"
WORKER_LOG="${LOG_DIR}/worker.log"
SUBMIT_JSON="${LOG_DIR}/submit.json"
STATUS_JSON="${LOG_DIR}/status.json"
UNKNOWN_JSON="${LOG_DIR}/unknown-status.json"

API_PID=""
WORKER_PID=""

COMPOSE_CMD=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")

# require_command verifies one CLI dependency exists on PATH.
require_command() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    log "missing required command: ${cmd}"
    exit 1
  fi
}

# preflight_checks validates local tooling and daemon readiness before test start.
preflight_checks() {
  require_command docker
  require_command go
  require_command curl
  if [[ "${WITH_UI_CHECKS}" == "true" ]]; then
    require_command npm
  fi

  if ! docker info >/dev/null 2>&1; then
    log "docker daemon is not reachable; start Docker Desktop and retry"
    exit 1
  fi
}

# run_ui_checks validates the React UI build path as part of e2e validation.
run_ui_checks() {
  if [[ ! -d "${UI_DIR}" ]]; then
    log "ui directory not found at ${UI_DIR}"
    exit 1
  fi

  if [[ -f "${UI_DIR}/package-lock.json" ]]; then
    log "running ui dependency install with npm ci"
    (
      cd "${UI_DIR}"
      npm ci
    )
  else
    log "running ui dependency install with npm install"
    (
      cd "${UI_DIR}"
      npm install
    )
  fi

  log "running ui production build"
  (
    cd "${UI_DIR}"
    npm run build
  )
}

# stop_process terminates one background process if still running.
stop_process() {
  local pid="$1"
  local name="$2"
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    log "stopping ${name} pid=${pid}"
    kill "${pid}" 2>/dev/null || true
    wait "${pid}" 2>/dev/null || true
  fi
}

# cleanup stops background processes and optionally tears down infra.
cleanup() {
  local exit_code=$?
  set +e

  stop_process "${API_PID}" "api"
  stop_process "${WORKER_PID}" "worker"

  if [[ "${KEEP_INFRA}" == "false" ]]; then
    if [[ "${PURGE_VOLUMES}" == "true" ]]; then
      log "tearing down docker compose services and volumes (--purge)"
      "${COMPOSE_CMD[@]}" down -v >/dev/null 2>&1 || true
    else
      log "tearing down docker compose services"
      "${COMPOSE_CMD[@]}" down >/dev/null 2>&1 || true
    fi
  else
    log "keeping docker compose services running (--keep-infra)"
  fi

  if [[ ${exit_code} -ne 0 ]]; then
    log "test failed; showing recent logs"
    log "api log: ${API_LOG}"
    tail -n 40 "${API_LOG}" 2>/dev/null || true
    log "worker log: ${WORKER_LOG}"
    tail -n 40 "${WORKER_LOG}" 2>/dev/null || true
  fi
}

trap cleanup EXIT INT TERM

# wait_for_api_health waits until /healthz reports status=ok.
wait_for_api_health() {
  local attempts=45
  local response=""
  for ((i=1; i<=attempts; i++)); do
    response="$(curl -s "${API_BASE_URL}/healthz" || true)"
    if [[ "${response}" == *'"status":"ok"'* ]]; then
      log "api health is ready"
      return 0
    fi
    sleep 1
  done
  log "api health did not become ready in time"
  return 1
}

# wait_for_status_path waits until worker-backed status returns 404 for unknown jobs.
wait_for_status_path() {
  local attempts=45
  local unknown_id="00000000-0000-0000-0000-000000000000"
  local code=""
  for ((i=1; i<=attempts; i++)); do
    code="$(curl -s -o "${UNKNOWN_JSON}" -w '%{http_code}' "${API_BASE_URL}/v1/jobs/${unknown_id}/status" || true)"
    if [[ "${code}" == "404" ]]; then
      log "worker status request-reply is ready"
      return 0
    fi
    sleep 1
  done
  log "status request-reply path did not become ready in time"
  return 1
}

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ENV_EXAMPLE}" "${ENV_FILE}"
  log "created ${ENV_FILE} from .env.example"
fi

preflight_checks

if [[ "${WITH_UI_CHECKS}" == "true" ]]; then
  run_ui_checks
else
  log "skipping ui checks (use --with-ui-checks to enable)"
fi

log "starting docker compose infrastructure"
"${COMPOSE_CMD[@]}" up -d
log "running connectivity smoke checks"
bash "${COMPOSE_DIR}/scripts/smoke-connectivity.sh"

log "ensuring isolated kafka topic exists (${TOPIC})"
"${COMPOSE_CMD[@]}" exec -T kafka kafka-topics \
  --bootstrap-server localhost:9092 \
  --create \
  --if-not-exists \
  --topic "${TOPIC}" \
  --partitions 1 \
  --replication-factor 1 >/dev/null

if [[ "${SKIP_UNIT_TESTS}" == "false" ]]; then
  log "running api unit tests"
  (
    cd "${API_DIR}"
    go test ./...
  )
  log "running worker unit tests"
  (
    cd "${WORKER_DIR}"
    go test ./...
  )
else
  log "skipping unit tests (--skip-unit-tests)"
fi

log "starting api process"
(
  cd "${API_DIR}"
  API_ADDR="${API_ADDR}" \
  KAFKA_TOPIC="${TOPIC}" \
  RABBITMQ_PROGRESS_REQUEST_QUEUE="${PROGRESS_QUEUE}" \
  go run . >"${API_LOG}" 2>&1
) &
API_PID="$!"

log "starting worker process"
(
  cd "${WORKER_DIR}"
  WORKER_KAFKA_TOPIC="${TOPIC}" \
  WORKER_KAFKA_GROUP_ID="${WORKER_GROUP_ID}" \
  RABBITMQ_PROGRESS_REQUEST_QUEUE="${PROGRESS_QUEUE}" \
  RABBITMQ_PROGRESS_CONSUMER_TAG="${PROGRESS_CONSUMER_TAG}" \
  WEATHER_PROVIDER="mock" \
  go run . >"${WORKER_LOG}" 2>&1
) &
WORKER_PID="$!"

wait_for_api_health
wait_for_status_path

log "submitting test job"
submit_code="$(
  curl -s -o "${SUBMIT_JSON}" -w '%{http_code}' \
    -X POST "${API_BASE_URL}/v1/jobs" \
    -H 'Content-Type: application/json' \
    -d '{"job_type":"weather","payload":{"city":"Austin","country_code":"US","units":"metric"}}'
)"
if [[ "${submit_code}" != "202" ]]; then
  log "job submit failed with http=${submit_code}"
  cat "${SUBMIT_JSON}"
  exit 1
fi

JOB_ID="$(sed -nE 's/.*"job_id":"([^"]+)".*/\1/p' "${SUBMIT_JSON}")"
if [[ -z "${JOB_ID}" ]]; then
  log "could not parse job_id from submit response"
  cat "${SUBMIT_JSON}"
  exit 1
fi
log "submitted job_id=${JOB_ID}"

final_state=""
final_progress=""
for ((i=1; i<=60; i++)); do
  status_code="$(curl -s -o "${STATUS_JSON}" -w '%{http_code}' "${API_BASE_URL}/v1/jobs/${JOB_ID}/status" || true)"
  if [[ "${status_code}" == "200" ]]; then
    final_state="$(sed -nE 's/.*"state":"([^"]+)".*/\1/p' "${STATUS_JSON}")"
    final_progress="$(sed -nE 's/.*"progress_percent":([0-9]+).*/\1/p' "${STATUS_JSON}")"
    log "status poll attempt=${i} state=${final_state:-unknown} progress=${final_progress:-unknown}"
    if [[ "${final_state}" == "completed" && "${final_progress}" == "100" ]]; then
      break
    fi
  fi
  sleep 1
done

if [[ "${final_state}" != "completed" || "${final_progress}" != "100" ]]; then
  log "job did not reach completed state in time"
  cat "${STATUS_JSON}" 2>/dev/null || true
  exit 1
fi

unknown_code="$(
  curl -s -o "${UNKNOWN_JSON}" -w '%{http_code}' \
    "${API_BASE_URL}/v1/jobs/00000000-0000-0000-0000-000000000000/status"
)"
if [[ "${unknown_code}" != "404" ]]; then
  log "expected unknown job status to return 404, got ${unknown_code}"
  cat "${UNKNOWN_JSON}"
  exit 1
fi

redis_state="$("${COMPOSE_CMD[@]}" exec -T redis redis-cli HGET "job:${JOB_ID}:status" state | tr -d '\r')"
redis_progress="$("${COMPOSE_CMD[@]}" exec -T redis redis-cli HGET "job:${JOB_ID}:status" progress_percent | tr -d '\r')"
if [[ "${redis_state}" != "completed" || "${redis_progress}" != "100" ]]; then
  log "redis validation failed state=${redis_state} progress=${redis_progress}"
  "${COMPOSE_CMD[@]}" exec -T redis redis-cli HGETALL "job:${JOB_ID}:status" || true
  exit 1
fi

mongo_count="$(
  "${COMPOSE_CMD[@]}" exec -T mongo mongosh --quiet --eval \
    "db.getSiblingDB('dtq').job_results.countDocuments({job_id:'${JOB_ID}'})" \
    | tr -d '\r' \
    | tail -n 1 \
    | tr -d '[:space:]'
)"
if [[ "${mongo_count}" != "1" ]]; then
  log "mongo validation failed expected=1 actual=${mongo_count}"
  "${COMPOSE_CMD[@]}" exec -T mongo mongosh --quiet --eval \
    "db.getSiblingDB('dtq').job_results.find({job_id:'${JOB_ID}'}).pretty()" || true
  exit 1
fi

kafka_msg="$(
  "${COMPOSE_CMD[@]}" exec -T kafka kafka-console-consumer \
    --bootstrap-server localhost:9092 \
    --topic "${TOPIC}" \
    --from-beginning \
    --max-messages 1 \
    --timeout-ms 10000 \
    | tr -d '\r'
)"
if [[ "${kafka_msg}" != *"${JOB_ID}"* ]]; then
  log "kafka validation failed to find job_id in consumed payload"
  printf '%s\n' "${kafka_msg}"
  exit 1
fi

log "current e2e test passed"
log "job_id=${JOB_ID}"
log "isolated_topic=${TOPIC}"
log "isolated_progress_queue=${PROGRESS_QUEUE}"
log "logs_dir=${LOG_DIR}"
