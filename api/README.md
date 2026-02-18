# API Service (Week 2)

Go API for Week 2 GraphQL and gRPC-integrated flow.

Implemented surface:
- `POST /graphql` (GraphQL mutation/query)
- `GET /graphql/ws` (GraphQL subscriptions over `graphql-transport-ws`)
- `GET /healthz`
- Legacy Week 1 REST job paths are disabled (`410 Gone`)

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
- `WORKER_GRPC_ADDR` (default `localhost:9090`)
- `WORKER_GRPC_DIAL_TIMEOUT` (default `5s`)
- `GRAPHQL_SUBSCRIPTION_POLL_INTERVAL` (default `750ms`)

## GraphQL Examples

Submit mutation:

```bash
curl -s -X POST http://localhost:8080/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query":"mutation SubmitJob($input: SubmitJobInput!){submitJob(input:$input){jobId state message}}",
    "variables":{
      "input":{
        "jobType":"weather",
        "payload":{"city":"Austin","country_code":"US","units":"metric"}
      }
    }
  }'
```

Status query:

```bash
curl -s -X POST http://localhost:8080/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query":"query JobStatus($jobId: ID!){jobStatus(jobId:$jobId){jobId state progressPercent message timestamp}}",
    "variables":{"jobId":"<job_id>"}
  }'
```
