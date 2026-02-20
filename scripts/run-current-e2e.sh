#!/usr/bin/env bash
set -euo pipefail

# run-current-e2e executes deterministic end-to-end validation for the latest Day N flow.
# It validates Week 2 runtime behavior and Week 4 Day 19 monitoring/reliability scaffolding checks.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_DIR="${ROOT_DIR}/infra/compose"
COMPOSE_FILE="${COMPOSE_DIR}/docker-compose.yml"
ENV_FILE="${COMPOSE_DIR}/.env"
ENV_EXAMPLE="${COMPOSE_DIR}/.env.example"
API_DIR="${ROOT_DIR}/api"
WORKER_DIR="${ROOT_DIR}/worker"
UI_DIR="${ROOT_DIR}/ui"
AKS_BASE_DIR="${ROOT_DIR}/infra/aks/base"
AKS_MONITORING_DIR="${ROOT_DIR}/infra/aks/monitoring"
AKS_DEPLOY_SCRIPT="${ROOT_DIR}/infra/aks/scripts/deploy-release.sh"
AZURE_ANSIBLE_DIR="${ROOT_DIR}/infra/azure/ansible"
JENKINSFILE_PATH="${ROOT_DIR}/Jenkinsfile"
WEEK4_EXECUTION_DOC="${ROOT_DIR}/docs/week-4-execution.md"

KEEP_INFRA=false
SKIP_UNIT_TESTS=false
PURGE_VOLUMES=false
WITH_UI_CHECKS=false
SKIP_WEEK4_CHECKS=false
SKIP_IMAGE_BUILD_CHECKS=false

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
  --skip-image-build-checks  Skip local Docker image build validation for api/worker/ui.
  --with-ui-checks     Run frontend validation (`npm install/ci` + `npm run build`).
  --skip-week4-checks  Skip Week 4 Day 19 scaffold checks (Jenkinsfile/AKS/Ansible/Monitoring).
  --skip-week3-checks  Alias for --skip-week4-checks.
  --purge              Remove compose volumes on teardown (`docker compose down -v`).
  -h, --help           Show this help message.

Behavior:
  1) Validates Week 4 Day 19 scaffolding (Jenkins/AKS contract wiring, monitoring manifests, deploy-helper dry run, Week 4 execution doc, AKS manifests/env wiring, worker reliability/shutdown env wiring, Ansible preflight when installed).
  2) Validates local container image builds for api/worker/ui (unless skipped).
  3) Starts compose infrastructure and smoke-checks it.
  4) Optionally runs frontend build checks (`--with-ui-checks`).
  5) Runs api/worker unit tests (unless skipped).
  6) Starts API and worker with isolated topic/queue settings.
  7) Submits a weather-profile job via GraphQL, validates subscription updates, and checks Redis/Mongo/Kafka.
  8) Tears down app processes and infra by default.
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
    --skip-image-build-checks)
      SKIP_IMAGE_BUILD_CHECKS=true
      shift
      ;;
    --with-ui-checks)
      WITH_UI_CHECKS=true
      shift
      ;;
    --skip-week4-checks)
      SKIP_WEEK4_CHECKS=true
      shift
      ;;
    --skip-week3-checks)
      SKIP_WEEK4_CHECKS=true
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
WORKER_GRPC_ADDR="${WORKER_GRPC_ADDR:-127.0.0.1:19090}"
WORKER_METRICS_ADDR="${WORKER_METRICS_ADDR:-127.0.0.1:19112}"
WORKER_METRICS_URL="http://${WORKER_METRICS_ADDR}"

LOG_DIR="${ROOT_DIR}/.tmp/e2e/${RUN_ID}"
mkdir -p "${LOG_DIR}"
API_LOG="${LOG_DIR}/api.log"
WORKER_LOG="${LOG_DIR}/worker.log"
SUBMIT_JSON="${LOG_DIR}/submit.json"
STATUS_JSON="${LOG_DIR}/status.json"
UNKNOWN_JSON="${LOG_DIR}/unknown-status.json"
API_METRICS_SNAPSHOT="${LOG_DIR}/api-metrics.txt"
WORKER_METRICS_SNAPSHOT="${LOG_DIR}/worker-metrics.txt"

API_PID=""
WORKER_PID=""
BUILT_IMAGES=()

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
  require_command node
  if [[ "${SKIP_WEEK4_CHECKS}" == "false" ]]; then
    require_command kubectl
  fi
  if [[ "${WITH_UI_CHECKS}" == "true" ]]; then
    require_command npm
  fi

  if ! docker info >/dev/null 2>&1; then
    log "docker daemon is not reachable; start Docker Desktop and retry"
    exit 1
  fi
}

# run_week4_day19_checks validates local cloud/monitoring/reliability scaffolding introduced for Week 4.
run_week4_day19_checks() {
  local aks_render_file="${LOG_DIR}/aks-render.yaml"
  local aks_monitoring_render_file="${LOG_DIR}/aks-monitoring-render.yaml"
  local ansible_vars_file="${LOG_DIR}/week3-preflight-vars.yml"
  local temp_ssh_pub="${LOG_DIR}/jenkins-temp.pub"
  local api_manifest="${AKS_BASE_DIR}/api-deployment.yaml"
  local worker_manifest="${AKS_BASE_DIR}/worker-deployment.yaml"
  local worker_service_manifest="${AKS_BASE_DIR}/worker-service.yaml"
  local worker_metrics_service_manifest="${AKS_BASE_DIR}/worker-metrics-service.yaml"
  local api_config_manifest="${AKS_BASE_DIR}/api-configmap.yaml"
  local worker_config_manifest="${AKS_BASE_DIR}/worker-configmap.yaml"
  local runtime_secret_manifest="${AKS_BASE_DIR}/runtime-secrets.yaml"
  local api_dockerfile="${API_DIR}/Dockerfile"
  local worker_dockerfile="${WORKER_DIR}/Dockerfile"
  local ui_dockerfile="${UI_DIR}/Dockerfile"
  local ui_nginx_conf="${UI_DIR}/nginx.conf"
  local deploy_script="${AKS_DEPLOY_SCRIPT}"
  local week4_execution_doc="${WEEK4_EXECUTION_DOC}"
  local required_stage

  if [[ ! -f "${JENKINSFILE_PATH}" ]]; then
    log "missing Jenkinsfile at ${JENKINSFILE_PATH}"
    exit 1
  fi

  for required_stage in "stage('Checkout')" "stage('Unit Tests')" "stage('UI Build')" "stage('Docker Build')" "stage('Push to ACR')" "stage('Deploy to AKS')"; do
    if ! grep -q "${required_stage}" "${JENKINSFILE_PATH}"; then
      log "Jenkinsfile missing required stage marker: ${required_stage}"
      exit 1
    fi
  done
  if grep -q "TODO:" "${JENKINSFILE_PATH}"; then
    log "Jenkinsfile still contains TODO placeholders; Day 19 pipeline wiring is incomplete"
    exit 1
  fi
  for required_cmd in \
    'for component in api worker ui; do' \
    'image_ref="${ACR_LOGIN_SERVER}/dtq-${component}:${IMAGE_TAG}"' \
    'docker build -f "${component}/Dockerfile" -t "${image_ref}" "${component}"' \
    'docker push "${image_ref}"' \
    "az login --service-principal" \
    "az aks get-credentials" \
    'bash infra/aks/scripts/deploy-release.sh'; do
    if ! grep -F -q "${required_cmd}" "${JENKINSFILE_PATH}"; then
      log "Jenkinsfile missing required Day 19 command marker: ${required_cmd}"
      exit 1
    fi
  done
  log "jenkinsfile stage and day19 command wiring check passed"

  for required_file in "${api_dockerfile}" "${worker_dockerfile}" "${ui_dockerfile}" "${ui_nginx_conf}"; do
    if [[ ! -f "${required_file}" ]]; then
      log "required Day 19 containerization file missing: ${required_file}"
      exit 1
    fi
  done
  log "dockerfile scaffold check passed"

  if [[ ! -x "${deploy_script}" ]]; then
    log "AKS deploy helper script missing or not executable: ${deploy_script}"
    exit 1
  fi
  for required_marker in \
    'dtq-%s:%s' \
    'kubectl apply -k "${AKS_BASE_DIR}"' \
    'set image deployment/dtq-api' \
    'set image deployment/dtq-worker' \
    'set image deployment/dtq-ui' \
    'rollout status deployment/dtq-api' \
    'rollout status deployment/dtq-worker' \
    'rollout status deployment/dtq-ui' \
    'DRY_RUN="${DRY_RUN:-false}"' \
    'if [[ "${DRY_RUN}" == "true" ]]; then' \
    'kubectl kustomize "${AKS_BASE_DIR}" >/dev/null'; do
    if ! grep -F -q "${required_marker}" "${deploy_script}"; then
      log "AKS deploy helper missing required Day 19 marker: ${required_marker}"
      exit 1
    fi
  done
  log "aks deploy helper contract check passed"

  if ! ACR_LOGIN_SERVER="example.azurecr.io" IMAGE_TAG="day19-check" K8S_NAMESPACE="dtq" DRY_RUN="true" bash "${deploy_script}" >/dev/null; then
    log "AKS deploy helper dry-run validation failed"
    exit 1
  fi
  log "aks deploy helper dry-run check passed"

  if [[ ! -f "${week4_execution_doc}" ]]; then
    log "Week 4 execution doc missing at ${week4_execution_doc}"
    exit 1
  fi
  for required_execution_line in "^# Week 4 Execution Brief" "^## Live Task Status" "^## Session Log \\(Append-Only\\)" "^## Handoff Snapshot"; do
    if ! grep -Eq "${required_execution_line}" "${week4_execution_doc}"; then
      log "Week 4 execution doc missing required section matching ${required_execution_line}"
      exit 1
    fi
  done
  log "week 4 execution doc check passed"

  if [[ ! -f "${AKS_BASE_DIR}/kustomization.yaml" ]]; then
    log "missing AKS kustomization at ${AKS_BASE_DIR}/kustomization.yaml"
    exit 1
  fi

  kubectl kustomize "${AKS_BASE_DIR}" >"${aks_render_file}"
  if [[ ! -s "${aks_render_file}" ]]; then
    log "AKS render output is empty"
    exit 1
  fi
  if ! grep -q '^kind: Deployment$' "${aks_render_file}" || ! grep -q '^kind: Service$' "${aks_render_file}" || ! grep -q '^kind: Namespace$' "${aks_render_file}"; then
    log "AKS render output missing expected Namespace/Deployment/Service resources"
    exit 1
  fi
  for required_manifest in \
    "${api_manifest}" \
    "${worker_manifest}" \
    "${worker_service_manifest}" \
    "${worker_metrics_service_manifest}" \
    "${api_config_manifest}" \
    "${worker_config_manifest}" \
    "${runtime_secret_manifest}"; do
    if [[ ! -f "${required_manifest}" ]]; then
      log "required Week 3 manifest is missing: ${required_manifest}"
      exit 1
    fi
  done
  for required_name in \
    "name: dtq-api-config" \
    "name: dtq-worker-config" \
    "name: dtq-runtime-secrets" \
    "name: dtq-worker-grpc" \
    "name: dtq-worker-metrics"; do
    if ! grep -q "${required_name}" "${aks_render_file}"; then
      log "AKS render output missing required resource marker: ${required_name}"
      exit 1
    fi
  done
  if ! grep -q 'name: dtq-api-config' "${api_manifest}" || ! grep -q 'name: dtq-runtime-secrets' "${api_manifest}" || ! grep -q 'value: "dtq-worker-grpc:9090"' "${api_manifest}"; then
    log "api deployment manifest is missing required env wiring references"
    exit 1
  fi
  if ! grep -q 'name: dtq-worker-config' "${worker_manifest}" || ! grep -q 'name: dtq-runtime-secrets' "${worker_manifest}" || ! grep -q 'name: WORKER_GRPC_ADDR' "${worker_manifest}" || ! grep -q 'name: WORKER_METRICS_ADDR' "${worker_manifest}"; then
    log "worker deployment manifest is missing required env wiring references"
    exit 1
  fi
  for required_worker_config in \
    "WORKER_STATUS_WRITE_MAX_ATTEMPTS" \
    "WORKER_STATUS_WRITE_INITIAL_BACKOFF" \
    "WORKER_STATUS_WRITE_MAX_BACKOFF" \
    "WORKER_RESULT_WRITE_MAX_ATTEMPTS" \
    "WORKER_RESULT_WRITE_INITIAL_BACKOFF" \
    "WORKER_RESULT_WRITE_MAX_BACKOFF" \
    "WORKER_SHUTDOWN_DRAIN_TIMEOUT"; do
    if ! grep -q "${required_worker_config}" "${worker_config_manifest}"; then
      log "worker config manifest is missing Day 19 reliability key: ${required_worker_config}"
      exit 1
    fi
  done
  if ! grep -q 'tcpSocket:' "${worker_manifest}"; then
    log "worker deployment manifest is missing tcp liveness/readiness probes"
    exit 1
  fi
  log "aks kustomize render and manifest wiring checks passed"

  if [[ ! -f "${AKS_MONITORING_DIR}/kustomization.yaml" ]]; then
    log "missing AKS monitoring kustomization at ${AKS_MONITORING_DIR}/kustomization.yaml"
    exit 1
  fi

  kubectl kustomize "${AKS_MONITORING_DIR}" >"${aks_monitoring_render_file}"
  if [[ ! -s "${aks_monitoring_render_file}" ]]; then
    log "AKS monitoring render output is empty"
    exit 1
  fi
  for required_monitoring_name in \
    "name: dtq-monitoring" \
    "name: dtq-prometheus-config" \
    "name: dtq-prometheus" \
    "name: dtq-grafana"; do
    if ! grep -q "${required_monitoring_name}" "${aks_monitoring_render_file}"; then
      log "AKS monitoring render missing required resource marker: ${required_monitoring_name}"
      exit 1
    fi
  done
  log "aks monitoring kustomize render checks passed"

  if command -v ansible-playbook >/dev/null 2>&1; then
    printf '%s\n' "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCyT2DAY19ValidationKey local-test" >"${temp_ssh_pub}"

    cat >"${ansible_vars_file}" <<EOF
azure_subscription_id: "00000000-0000-0000-0000-000000000000"
azure_client_id: "00000000-0000-0000-0000-000000000000"
azure_client_secret: "local-test-secret"
azure_tenant_id: "00000000-0000-0000-0000-000000000000"
azure_location: "eastus"
azure_resource_group: "dtq-week3-rg"
azure_acr_name: "dtqweek3acr"
azure_aks_name: "dtq-week3-aks"
azure_aks_node_count: 2
azure_jenkins_vm_name: "dtq-jenkins-vm"
azure_jenkins_admin_username: "jenkinsadmin"
azure_jenkins_ssh_public_key_path: "${temp_ssh_pub}"
EOF

    ansible-playbook -i localhost, "${AZURE_ANSIBLE_DIR}/playbooks/day12-preflight.yml" --syntax-check >/dev/null
    ansible-playbook -i localhost, "${AZURE_ANSIBLE_DIR}/playbooks/day12-preflight.yml" -e "@${ansible_vars_file}" >/dev/null
    log "ansible week3 preflight check passed"
  else
    log "ansible-playbook not installed; skipping ansible week3 preflight check"
  fi
}

# run_image_build_checks validates local Dockerfiles by building all service images.
run_image_build_checks() {
  local image_tag="day19-local-${RUN_ID}"
  local component
  local image_ref

  for component in api worker ui; do
    image_ref="dtq-${component}:${image_tag}"
    docker build -f "${ROOT_DIR}/${component}/Dockerfile" -t "${image_ref}" "${ROOT_DIR}/${component}" >/dev/null
    BUILT_IMAGES+=("${image_ref}")
    log "docker image build check passed component=${component} image=${image_ref}"
  done
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

  for image in "${BUILT_IMAGES[@]:-}"; do
    if [[ -z "${image}" ]]; then
      continue
    fi
    docker image rm -f "${image}" >/dev/null 2>&1 || true
  done

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

# graphql_status_query executes GraphQL status query for one job_id and writes response body.
graphql_status_query() {
  local job_id="$1"
  local out_file="$2"
  local payload

  payload="$(printf '{"query":"query JobStatus($jobId: ID!){jobStatus(jobId:$jobId){jobId state progressPercent message timestamp}}","variables":{"jobId":"%s"}}' "${job_id}")"
  curl -s -o "${out_file}" -w '%{http_code}' \
    -X POST "${API_BASE_URL}/graphql" \
    -H 'Content-Type: application/json' \
    -d "${payload}"
}

# wait_for_graphql_path waits until GraphQL query path responds with not_found for unknown jobs.
wait_for_graphql_path() {
  local attempts=45
  local unknown_id="00000000-0000-0000-0000-000000000000"
  local code=""
  for ((i=1; i<=attempts; i++)); do
    code="$(graphql_status_query "${unknown_id}" "${UNKNOWN_JSON}" || true)"
    if [[ "${code}" == "200" ]] && grep -q '"state":"not_found"' "${UNKNOWN_JSON}"; then
      log "graphql query path is ready"
      return 0
    fi
    sleep 1
  done
  log "graphql query path did not become ready in time"
  return 1
}

# wait_for_metrics_endpoint waits until one metrics endpoint contains a required metric marker.
wait_for_metrics_endpoint() {
  local endpoint_url="$1"
  local required_marker="$2"
  local endpoint_name="$3"
  local out_file="$4"
  local attempts=45

  for ((i=1; i<=attempts; i++)); do
    if curl -s "${endpoint_url}" >"${out_file}" && grep -q "${required_marker}" "${out_file}"; then
      log "${endpoint_name} metrics endpoint is ready"
      return 0
    fi
    sleep 1
  done

  log "${endpoint_name} metrics endpoint did not become ready in time"
  return 1
}

# run_graphql_subscription_check validates GraphQL websocket subscription delivery through terminal state.
run_graphql_subscription_check() {
  local job_id="$1"
  API_BASE_URL="${API_BASE_URL}" JOB_ID="${job_id}" node <<'NODE'
const apiBaseUrl = process.env.API_BASE_URL;
const jobId = process.env.JOB_ID;

if (!apiBaseUrl || !jobId) {
  console.error("missing API_BASE_URL or JOB_ID");
  process.exit(1);
}
if (typeof WebSocket === "undefined") {
  console.error("node runtime does not expose global WebSocket");
  process.exit(1);
}

const wsUrl = apiBaseUrl.replace(/^http:/, "ws:").replace(/^https:/, "wss:") + "/graphql/ws";
const ws = new WebSocket(wsUrl, "graphql-transport-ws");

let acked = false;
let events = 0;
let completed = false;

const timeout = setTimeout(() => {
  console.error("subscription timed out without terminal event");
  process.exit(1);
}, 30000);

ws.onopen = () => {
  ws.send(JSON.stringify({ type: "connection_init" }));
};

ws.onmessage = (event) => {
  let message;
  try {
    message = JSON.parse(String(event.data));
  } catch (err) {
    console.error("subscription received invalid JSON:", err);
    process.exit(1);
  }

  if (message.type === "connection_ack") {
    acked = true;
    ws.send(
      JSON.stringify({
        id: "job-progress-check",
        type: "subscribe",
        payload: {
          query:
            "subscription JobProgress($jobId: ID!){jobProgress(jobId:$jobId){jobId state progressPercent message timestamp}}",
          variables: { jobId },
        },
      })
    );
    return;
  }

  if (message.type === "next") {
    const progress = message?.payload?.data?.jobProgress;
    if (!progress) {
      console.error("subscription next payload missing data.jobProgress");
      process.exit(1);
    }
    events += 1;
    if (["completed", "failed", "not_found"].includes(String(progress.state || "").toLowerCase())) {
      completed = true;
      ws.send(JSON.stringify({ id: "job-progress-check", type: "complete" }));
      ws.close();
    }
    return;
  }

  if (message.type === "error") {
    console.error("subscription protocol error:", JSON.stringify(message.payload));
    process.exit(1);
  }

  if (message.type === "complete" && completed && events > 0) {
    clearTimeout(timeout);
    process.exit(0);
  }
};

ws.onclose = () => {
  if (!acked) {
    console.error("subscription socket closed before connection_ack");
    process.exit(1);
  }
  if (!completed || events === 0) {
    console.error("subscription closed before terminal status event");
    process.exit(1);
  }
  clearTimeout(timeout);
  process.exit(0);
};

ws.onerror = (event) => {
  console.error("subscription websocket error", event?.message || "");
  process.exit(1);
};
NODE
}

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ENV_EXAMPLE}" "${ENV_FILE}"
  log "created ${ENV_FILE} from .env.example"
fi

preflight_checks

if [[ "${SKIP_WEEK4_CHECKS}" == "false" ]]; then
  log "running week 4 day 19 scaffold checks"
  run_week4_day19_checks
else
  log "skipping week 4 checks (--skip-week4-checks/--skip-week3-checks)"
fi

if [[ "${SKIP_IMAGE_BUILD_CHECKS}" == "false" ]]; then
  log "running local image build checks"
  run_image_build_checks
else
  log "skipping local image build checks (--skip-image-build-checks)"
fi

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
  REDIS_ADDR="127.0.0.1:6379" \
  RABBITMQ_URL="amqp://guest:guest@127.0.0.1:5672/" \
  RABBITMQ_PROGRESS_REQUEST_QUEUE="${PROGRESS_QUEUE}" \
  WORKER_GRPC_ADDR="${WORKER_GRPC_ADDR}" \
  go run . >"${API_LOG}" 2>&1
) &
API_PID="$!"

log "starting worker process"
(
  cd "${WORKER_DIR}"
  WORKER_KAFKA_TOPIC="${TOPIC}" \
  WORKER_KAFKA_GROUP_ID="${WORKER_GROUP_ID}" \
  REDIS_ADDR="127.0.0.1:6379" \
  MONGO_URI="mongodb://127.0.0.1:27017" \
  RABBITMQ_URL="amqp://guest:guest@127.0.0.1:5672/" \
  RABBITMQ_PROGRESS_REQUEST_QUEUE="${PROGRESS_QUEUE}" \
  RABBITMQ_PROGRESS_CONSUMER_TAG="${PROGRESS_CONSUMER_TAG}" \
  WORKER_GRPC_ADDR="${WORKER_GRPC_ADDR}" \
  WORKER_METRICS_ADDR="${WORKER_METRICS_ADDR}" \
  WEATHER_PROVIDER="mock" \
  go run . >"${WORKER_LOG}" 2>&1
) &
WORKER_PID="$!"

wait_for_api_health
wait_for_graphql_path
wait_for_metrics_endpoint "${API_BASE_URL}/metrics" "dtq_api_graphql_http_requests_total" "api" "${API_METRICS_SNAPSHOT}"
wait_for_metrics_endpoint "${WORKER_METRICS_URL}/metrics" "dtq_worker_job_attempts_total" "worker" "${WORKER_METRICS_SNAPSHOT}"

log "submitting test job via graphql mutation"
submit_payload='{"query":"mutation SubmitJob($input: SubmitJobInput!){submitJob(input:$input){jobId traceId jobType state submittedAt message}}","variables":{"input":{"jobType":"weather","payload":{"city":"Austin","country_code":"US","units":"metric"}}}}'
submit_code="$(
  curl -s -o "${SUBMIT_JSON}" -w '%{http_code}' \
    -X POST "${API_BASE_URL}/graphql" \
    -H 'Content-Type: application/json' \
    -d "${submit_payload}"
)"
if [[ "${submit_code}" != "200" ]]; then
  log "job submit failed with http=${submit_code}"
  cat "${SUBMIT_JSON}"
  exit 1
fi

JOB_ID="$(sed -nE 's/.*"jobId":"([^"]+)".*/\1/p' "${SUBMIT_JSON}")"
if [[ -z "${JOB_ID}" ]]; then
  log "could not parse job_id from submit response"
  cat "${SUBMIT_JSON}"
  exit 1
fi
log "submitted job_id=${JOB_ID}"

log "validating graphql subscription progress stream"
if ! run_graphql_subscription_check "${JOB_ID}"; then
  log "graphql subscription check failed"
  exit 1
fi

log "querying final status via graphql"
status_code="$(graphql_status_query "${JOB_ID}" "${STATUS_JSON}" || true)"
if [[ "${status_code}" != "200" ]]; then
  log "graphql status query failed with http=${status_code}"
  cat "${STATUS_JSON}"
  exit 1
fi

final_state="$(sed -nE 's/.*"state":"([^"]+)".*/\1/p' "${STATUS_JSON}")"
final_progress="$(sed -nE 's/.*"progressPercent":([0-9]+).*/\1/p' "${STATUS_JSON}")"
if [[ "${final_state}" != "completed" || "${final_progress}" != "100" ]]; then
  log "graphql status validation failed state=${final_state:-unknown} progress=${final_progress:-unknown}"
  cat "${STATUS_JSON}"
  exit 1
fi

unknown_code="$(graphql_status_query "00000000-0000-0000-0000-000000000000" "${UNKNOWN_JSON}" || true)"
if [[ "${unknown_code}" != "200" ]] || ! grep -q '"state":"not_found"' "${UNKNOWN_JSON}"; then
  log "expected unknown job graphql query to return state=not_found, got http=${unknown_code}"
  cat "${UNKNOWN_JSON}"
  exit 1
fi

if ! curl -s "${API_BASE_URL}/metrics" >"${API_METRICS_SNAPSHOT}"; then
  log "api metrics validation failed: could not fetch /metrics"
  exit 1
fi
if ! curl -s "${WORKER_METRICS_URL}/metrics" >"${WORKER_METRICS_SNAPSHOT}"; then
  log "worker metrics validation failed: could not fetch /metrics"
  exit 1
fi
if ! grep -q '^dtq_worker_status_write_retries_total ' "${WORKER_METRICS_SNAPSHOT}"; then
  log "worker metrics validation failed: dtq_worker_status_write_retries_total not found"
  exit 1
fi
if ! grep -q '^dtq_worker_result_write_retries_total ' "${WORKER_METRICS_SNAPSHOT}"; then
  log "worker metrics validation failed: dtq_worker_result_write_retries_total not found"
  exit 1
fi
if ! grep -q '^dtq_worker_shutdown_drain_completions_total ' "${WORKER_METRICS_SNAPSHOT}"; then
  log "worker metrics validation failed: dtq_worker_shutdown_drain_completions_total not found"
  exit 1
fi
if ! grep -q '^dtq_worker_shutdown_drain_timeouts_total ' "${WORKER_METRICS_SNAPSHOT}"; then
  log "worker metrics validation failed: dtq_worker_shutdown_drain_timeouts_total not found"
  exit 1
fi

api_submit_total="$(awk '$1=="dtq_api_job_submissions_total"{print $2}' "${API_METRICS_SNAPSHOT}" | tail -n1)"
worker_completed_total="$(awk '$1=="dtq_worker_job_completed_total"{print $2}' "${WORKER_METRICS_SNAPSHOT}" | tail -n1)"
if [[ -z "${api_submit_total}" || -z "${worker_completed_total}" ]]; then
  log "metrics validation failed: required metric lines were not found"
  exit 1
fi
if ! [[ "${api_submit_total}" =~ ^[0-9]+$ ]] || ! [[ "${worker_completed_total}" =~ ^[0-9]+$ ]]; then
  log "metrics validation failed: metric values are not integers api_submit_total=${api_submit_total} worker_completed_total=${worker_completed_total}"
  exit 1
fi
if (( api_submit_total < 1 || worker_completed_total < 1 )); then
  log "metrics validation failed: expected submission/completion counters >=1 api_submit_total=${api_submit_total} worker_completed_total=${worker_completed_total}"
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
