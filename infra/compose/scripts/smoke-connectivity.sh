#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.yml"
COMPOSE_CMD=(docker compose -f "${COMPOSE_FILE}")

SERVICES=(kafka rabbitmq redis mongo)
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-120}"
HEALTH_POLL_INTERVAL_SECONDS="${HEALTH_POLL_INTERVAL_SECONDS:-2}"
CONNECTIVITY_RETRIES="${CONNECTIVITY_RETRIES:-30}"
CONNECTIVITY_RETRY_DELAY_SECONDS="${CONNECTIVITY_RETRY_DELAY_SECONDS:-2}"

# wait_for_service_health waits for one container to become healthy/running.
wait_for_service_health() {
  local svc="$1"
  local elapsed=0
  local cid=""
  local health=""

  while (( elapsed < HEALTH_TIMEOUT_SECONDS )); do
    cid="$("${COMPOSE_CMD[@]}" ps -q "${svc}" 2>/dev/null || true)"
    if [[ -n "${cid}" ]]; then
      health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${cid}" 2>/dev/null || true)"
      if [[ "${health}" == "healthy" || "${health}" == "running" ]]; then
        echo "Service '${svc}' is ${health}."
        return 0
      fi
      echo "Waiting for '${svc}' (current status: ${health:-unknown})..."
    else
      echo "Waiting for '${svc}' container to start..."
    fi

    sleep "${HEALTH_POLL_INTERVAL_SECONDS}"
    elapsed=$((elapsed + HEALTH_POLL_INTERVAL_SECONDS))
  done

  echo "ERROR: service '${svc}' did not become healthy/running within ${HEALTH_TIMEOUT_SECONDS}s."
  return 1
}

# retry_command runs one command until it succeeds or retry budget is exhausted.
retry_command() {
  local description="$1"
  shift

  local attempt=1
  while (( attempt <= CONNECTIVITY_RETRIES )); do
    if "$@" >/dev/null 2>&1; then
      echo "${description} check passed."
      return 0
    fi
    sleep "${CONNECTIVITY_RETRY_DELAY_SECONDS}"
    attempt=$((attempt + 1))
  done

  echo "ERROR: ${description} check failed after ${CONNECTIVITY_RETRIES} attempts."
  return 1
}

# check_redis_ping verifies Redis responds with a PONG.
check_redis_ping() {
  local redis_ping=""
  redis_ping="$("${COMPOSE_CMD[@]}" exec -T redis redis-cli ping | tr -d '\r')"
  [[ "${redis_ping}" == "PONG" ]]
}

# check_mongo_ping verifies MongoDB ping command returns 1.
check_mongo_ping() {
  local mongo_ping=""
  mongo_ping="$("${COMPOSE_CMD[@]}" exec -T mongo mongosh --quiet --eval 'db.runCommand({ ping: 1 }).ok' | tr -d '\r')"
  [[ "${mongo_ping}" == "1" ]]
}

echo "Checking container health..."
for svc in "${SERVICES[@]}"; do
  wait_for_service_health "${svc}"
done
echo "All services are healthy/running."

echo "Running protocol-level connectivity checks..."
retry_command "Kafka" "${COMPOSE_CMD[@]}" exec -T kafka kafka-topics --bootstrap-server localhost:9092 --list
retry_command "RabbitMQ" "${COMPOSE_CMD[@]}" exec -T rabbitmq rabbitmq-diagnostics -q ping
retry_command "Redis" check_redis_ping
retry_command "MongoDB" check_mongo_ping

echo "Connectivity smoke test passed."
