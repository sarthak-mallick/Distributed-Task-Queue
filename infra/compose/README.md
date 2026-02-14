# Local Infrastructure (Week 1)

This stack runs the core Week 1 dependencies:
- Kafka (KRaft mode)
- RabbitMQ (management UI enabled)
- Redis
- MongoDB

## Quickstart

```bash
cp infra/compose/.env.example infra/compose/.env
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env up -d
```

## Health + Connectivity Check

```bash
bash infra/compose/scripts/smoke-connectivity.sh
```

## One-Command Day 1 Validation

```bash
bash infra/compose/scripts/bootstrap-and-smoke.sh
```

## Endpoints
- Kafka external: `localhost:9094`
- RabbitMQ AMQP: `localhost:5672`
- RabbitMQ UI: `http://localhost:15672`
- Redis: `localhost:6379`
- MongoDB: `localhost:27017`

## Shutdown

```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down
```

To remove persistent data volumes:

```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down -v
```
