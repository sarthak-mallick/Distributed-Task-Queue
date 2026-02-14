# Distributed-Task-Queue

A distributed task queue where users submit jobs via a web UI, workers process them, and users can monitor real-time progress.

## Week 1 Layout
- `api/` - temporary Go REST API (submit + status)
- `worker/` - Go Kafka consumer and progress responder
- `ui/` - basic React submit/status interface
- `infra/compose/` - local Kafka/RabbitMQ/Redis/Mongo stack
- `contracts/` - message/data contracts
- `docs/` - canonical spec and execution plans

## Day 1 Quickstart
```bash
cp infra/compose/.env.example infra/compose/.env
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env up -d
bash infra/compose/scripts/smoke-connectivity.sh
```

Or run the one-command bootstrap + validation:

```bash
bash infra/compose/scripts/bootstrap-and-smoke.sh
```
