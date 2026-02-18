#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
ENV_EXAMPLE="${ROOT_DIR}/.env.example"
COMPOSE_FILE="${ROOT_DIR}/docker-compose.yml"
COMPOSE_CMD=(docker compose -f "${COMPOSE_FILE}" --env-file "${ENV_FILE}")

CANONICAL_JOB_TYPES_DEFAULT="weather,quote,exchange_rate,github_user"
KAFKA_TOPIC_TEMPLATE_DEFAULT="jobs.%s.v1"
KAFKA_TOPIC_PARTITIONS_DEFAULT="1"
KAFKA_TOPIC_REPLICATION_FACTOR_DEFAULT="1"

# trim removes leading/trailing whitespace for one value.
trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

# contains_topic reports whether a topic already exists in one shell array.
contains_topic() {
  local target="$1"
  shift
  local existing=""
  for existing in "$@"; do
    if [[ "${existing}" == "${target}" ]]; then
      return 0
    fi
  done
  return 1
}

if [[ ! -f "${ENV_FILE}" ]]; then
  cp "${ENV_EXAMPLE}" "${ENV_FILE}"
  echo "Created ${ENV_FILE} from .env.example"
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

job_types_csv="$(trim "${CANONICAL_JOB_TYPES:-${CANONICAL_JOB_TYPES_DEFAULT}}")"
topic_template="$(trim "${KAFKA_TOPIC_TEMPLATE:-${KAFKA_TOPIC_TEMPLATE_DEFAULT}}")"
partitions="$(trim "${KAFKA_TOPIC_PARTITIONS:-${KAFKA_TOPIC_PARTITIONS_DEFAULT}}")"
replication_factor="$(trim "${KAFKA_TOPIC_REPLICATION_FACTOR:-${KAFKA_TOPIC_REPLICATION_FACTOR_DEFAULT}}")"

if [[ -z "${job_types_csv}" ]]; then
  echo "ERROR: CANONICAL_JOB_TYPES resolved to an empty value."
  exit 1
fi
if [[ -z "${topic_template}" ]]; then
  echo "ERROR: KAFKA_TOPIC_TEMPLATE resolved to an empty value."
  exit 1
fi

echo "Ensuring canonical Kafka topics exist..."
IFS=',' read -r -a raw_job_types <<< "${job_types_csv}"
topics=()
for raw_job_type in "${raw_job_types[@]}"; do
  job_type="$(trim "${raw_job_type}")"
  if [[ -z "${job_type}" ]]; then
    continue
  fi

  topic="${topic_template}"
  if [[ "${topic_template}" == *"%s"* ]]; then
    topic="${topic_template//%s/${job_type}}"
  fi

  if contains_topic "${topic}" "${topics[@]}"; then
    continue
  fi

  topics+=("${topic}")
  "${COMPOSE_CMD[@]}" exec -T kafka kafka-topics \
    --bootstrap-server localhost:9092 \
    --create \
    --if-not-exists \
    --topic "${topic}" \
    --partitions "${partitions}" \
    --replication-factor "${replication_factor}" >/dev/null
  echo "Topic ready: ${topic}"
done

echo "Canonical Kafka topic bootstrap complete."
