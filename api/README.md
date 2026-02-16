# API Service (Week 1)

Temporary Go REST API for Week 1.

Implemented Day 4 endpoints:
- `POST /v1/jobs` (generic)
- `GET /healthz`
- `GET /v1/jobs/{job_id}/status` (RabbitMQ request-reply backed)

## Run

```bash
cd api
go run .
```

## Environment

- `API_ADDR` (default `:8080`)
- `KAFKA_BROKERS` (default `localhost:9094`, comma-separated)
- `KAFKA_TOPIC` (optional fixed topic override for all job types)
- `KAFKA_TOPIC_TEMPLATE` (default `jobs.%s.v1`, where `%s` = job type)
- `REDIS_ADDR` (default `localhost:6379`)
- `REDIS_USERNAME` (optional)
- `REDIS_PASSWORD` (optional)
- `REDIS_DB` (default `0`)
- `STATUS_TTL` (default `24h`)
- `REQUEST_TIMEOUT` (default `5s`)
- `RABBITMQ_URL` (default `amqp://guest:guest@localhost:5672/`)
- `RABBITMQ_PROGRESS_REQUEST_QUEUE` (default `progress.check.request.v1`)
- `PROGRESS_REPLY_TIMEOUT` (default `5s`)

## Generic Submit Example

```bash
curl -X POST http://localhost:8080/v1/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "job_type": "weather",
    "payload": {
      "city": "Austin",
      "country_code": "US",
      "units": "metric"
    }
  }'
```

Currently supported `job_type` values:
- `weather`
- `quote`
- `exchange_rate`
- `github_user`

## Status Example

```bash
curl -s http://localhost:8080/v1/jobs/<job_id>/status | jq
```
