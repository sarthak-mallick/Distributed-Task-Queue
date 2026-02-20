# Worker Service (Week 2)

Go worker service for Week 2 migration.

Current implementation scope:
- Consumes Kafka jobs using a consumer group
- Processes weather jobs (plus generic non-weather placeholders)
- Maintains Redis progress milestones and Mongo durable results
- Serves RabbitMQ request-reply progress path (Week 1 compatibility)
- Serves Week 2 gRPC API-worker contract:
  - `GetJobStatus`
  - `SubscribeJobProgress`
- Retries transient weather provider failures with exponential backoff

## Run Locally

```bash
cd worker
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
- `WORKER_FETCH_ERROR_BACKOFF` (default: `500ms`)
- `WORKER_PROCESS_TIMEOUT` (default: `45s`)
- `WORKER_COMMIT_TIMEOUT` (default: `5s`)
- `WORKER_STATUS_WRITE_MAX_ATTEMPTS` (default: `3`)
- `WORKER_STATUS_WRITE_INITIAL_BACKOFF` (default: `200ms`)
- `WORKER_STATUS_WRITE_MAX_BACKOFF` (default: `2s`)
- `WORKER_RESULT_WRITE_MAX_ATTEMPTS` (default: `3`)
- `WORKER_RESULT_WRITE_INITIAL_BACKOFF` (default: `200ms`)
- `WORKER_RESULT_WRITE_MAX_BACKOFF` (default: `2s`)
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
- `RABBITMQ_PROGRESS_RECONNECT_BACKOFF` (default: `2s`)
- `WORKER_GRPC_ADDR` (default: `:9090`)
- `WORKER_METRICS_ADDR` (default: `:2112`)
- `WORKER_GRPC_POLL_INTERVAL` (default: `750ms`)
- `WEATHER_PROVIDER` (`openmeteo` or `mock`, default: `openmeteo`)
- `WEATHER_HTTP_TIMEOUT` (default: `8s`)
- `WEATHER_USE_MOCK_FALLBACK` (default: `true`)
- `WEATHER_GEOCODING_URL` (default Open-Meteo geocoding URL)
- `WEATHER_FORECAST_URL` (default Open-Meteo forecast URL)
- `WEATHER_MAX_ATTEMPTS` (default: `3`)
- `WEATHER_RETRY_INITIAL_BACKOFF` (default: `500ms`)
- `WEATHER_RETRY_MAX_BACKOFF` (default: `4s`)
