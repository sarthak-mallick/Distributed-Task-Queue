#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
ENV_EXAMPLE="${ROOT_DIR}/.env.example"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.yml"

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ENV_EXAMPLE}" "${ENV_FILE}"
  echo "Created ${ENV_FILE} from .env.example"
fi

docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}" up -d
bash "${ROOT_DIR}/scripts/smoke-connectivity.sh"
bash "${ROOT_DIR}/scripts/ensure-kafka-topics.sh"

echo "Bootstrap, smoke checks, and Kafka topic bootstrap passed."
