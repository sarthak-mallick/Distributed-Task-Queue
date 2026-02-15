# Contracts

Message and data contracts for Kafka, Redis, MongoDB, and RabbitMQ live here.

## Week 1 Contract Artifacts

- API generic submit request:
  - `contracts/api/job-submit-request-v1.schema.json`
  - `contracts/api/job-submit-request-v1.example.json`
- Kafka generic job envelope (preferred):
  - `contracts/kafka/job-v1.schema.json`
  - `contracts/kafka/job-v1.example.json`
- Kafka job-type payload profile (weather profile currently used for testing):
  - `contracts/kafka/weather-job-v1.schema.json`
  - `contracts/kafka/weather-job-v1.example.json`
- Redis job status hash model:
  - `contracts/redis/job-status-v1.schema.json`
  - `contracts/redis/job-status-v1.example.json`
- MongoDB job result document:
  - `contracts/mongo/job-results-v1.schema.json`
  - `contracts/mongo/job-results-v1.example.json`
- RabbitMQ progress request/reply:
  - `contracts/rabbitmq/progress-check-request-v1.schema.json`
  - `contracts/rabbitmq/progress-check-request-v1.example.json`
  - `contracts/rabbitmq/progress-check-reply-v1.schema.json`
  - `contracts/rabbitmq/progress-check-reply-v1.example.json`

Canonical references:
- `docs/week-1-execution.md`
- `docs/project-spec.md`
