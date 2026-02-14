#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.yml"
COMPOSE_CMD=(docker compose -f "${COMPOSE_FILE}")

SERVICES=(kafka rabbitmq redis mongo)

echo "Checking container health..."
for svc in "${SERVICES[@]}"; do
  cid="$("${COMPOSE_CMD[@]}" ps -q "${svc}")"
  if [[ -z "${cid}" ]]; then
    echo "ERROR: service '${svc}' is not running."
    exit 1
  fi

  health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${cid}")"
  if [[ "${health}" != "healthy" && "${health}" != "running" ]]; then
    echo "ERROR: service '${svc}' status is '${health}' (expected healthy/running)."
    exit 1
  fi
done
echo "All services are healthy/running."

echo "Running protocol-level connectivity checks..."
"${COMPOSE_CMD[@]}" exec -T kafka kafka-topics --bootstrap-server localhost:9092 --list > /dev/null
"${COMPOSE_CMD[@]}" exec -T rabbitmq rabbitmq-diagnostics -q ping > /dev/null

redis_ping="$("${COMPOSE_CMD[@]}" exec -T redis redis-cli ping | tr -d '\r')"
if [[ "${redis_ping}" != "PONG" ]]; then
  echo "ERROR: Redis ping returned '${redis_ping}'"
  exit 1
fi

mongo_ping="$("${COMPOSE_CMD[@]}" exec -T mongo mongosh --quiet --eval 'db.runCommand({ ping: 1 }).ok' | tr -d '\r')"
if [[ "${mongo_ping}" != "1" ]]; then
  echo "ERROR: Mongo ping returned '${mongo_ping}'"
  exit 1
fi

echo "Connectivity smoke test passed."
