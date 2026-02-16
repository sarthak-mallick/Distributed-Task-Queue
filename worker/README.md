# Worker Service (Week 1)

Go worker service for Week 1.

Current implementation scope (worker-side `W1-008` to `W1-011`):
- Consumes Kafka jobs using a consumer group
- Routes messages by `job_type` (weather profile first, extensible handlers)
- Uses manual offset commits (`FetchMessage` + `CommitMessages`)
- Commits only after processing outcome:
  - success: commit
  - invalid/unsupported message: drop + commit
  - handler error: no commit (message can retry)
- Writes Redis progress/status updates for milestones:
  - `running:20` -> `running:50` -> `running:80` -> `completed:100`
- Processes weather jobs using provider abstraction (`openmeteo` or `mock`)
- Persists final results idempotently in MongoDB `job_results` (unique `job_id`)
- Handles RabbitMQ progress request-reply on `progress.check.request.v1` with AMQP `correlation_id` echo and `reply_to` routing

## Run Locally

```bash
cd worker
go mod tidy
go test ./...
go run .
```

## Environment Variables

- `WORKER_KAFKA_BROKERS` (default: `localhost:9094`)
- `WORKER_KAFKA_GROUP_ID` (default: `dtq-worker-v1`)
- `WORKER_JOB_TYPES` (default: `weather`)
- `WORKER_KAFKA_TOPIC` (optional single-topic override)
- `WORKER_KAFKA_TOPIC_TEMPLATE` (default: `jobs.%s.v1`)
- `WORKER_FETCH_MIN_BYTES` (default: `1`)
- `WORKER_FETCH_MAX_BYTES` (default: `10485760`)
- `WORKER_FETCH_MAX_WAIT` (default: `1s`)
- `WORKER_PROCESS_TIMEOUT` (default: `45s`)
- `WORKER_COMMIT_TIMEOUT` (default: `5s`)
- `REDIS_ADDR` (default: `localhost:6379`)
- `REDIS_USERNAME` (optional)
- `REDIS_PASSWORD` (optional)
- `REDIS_DB` (default: `0`)
- `STATUS_TTL` (default: `24h`)
- `MONGO_URI` (default: `mongodb://localhost:27017`)
- `MONGO_DB` (default: `dtq`)
- `MONGO_COLLECTION` (default: `job_results`)
- `MONGO_CONNECT_TIMEOUT` (default: `10s`)
- `RABBITMQ_URL` (default: `amqp://guest:guest@localhost:5672/`)
- `RABBITMQ_PROGRESS_REQUEST_QUEUE` (default: `progress.check.request.v1`)
- `RABBITMQ_PROGRESS_CONSUMER_TAG` (default: `dtq-worker-progress-v1`)
- `RABBITMQ_PROGRESS_PREFETCH` (default: `20`)
- `WEATHER_PROVIDER` (`openmeteo` or `mock`, default: `openmeteo`)
- `WEATHER_HTTP_TIMEOUT` (default: `8s`)
- `WEATHER_USE_MOCK_FALLBACK` (default: `true`)
- `WEATHER_GEOCODING_URL` (default Open-Meteo geocoding URL)
- `WEATHER_FORECAST_URL` (default Open-Meteo forecast URL)
