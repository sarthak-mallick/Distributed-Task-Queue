# Local Runbook

Last updated: 2026-02-21

Use this guide to run the full application locally (infra + worker + API + UI).

## Prerequisites

- Docker Desktop (or Docker daemon) running
- Go 1.23+
- Node.js 20+ and npm

## Start Local Stack

1. Start infra and bootstrap canonical topics:

```bash
cp infra/compose/.env.example infra/compose/.env
bash infra/compose/scripts/bootstrap-and-smoke.sh
```

2. Start worker (Terminal 1):

```bash
cd worker
go run .
```

3. Start API (Terminal 2):

```bash
cd api
go run .
```

4. Start UI (Terminal 3):

```bash
cd ui
npm install
npm run dev
```

5. Open [http://localhost:5173](http://localhost:5173) and use API base URL `http://localhost:8080`.

## Sample GraphQL Payloads

Endpoint:

```text
http://localhost:8080/graphql
```

Submit mutation:

```json
{
  "query": "mutation SubmitJob($input: SubmitJobInput!){submitJob(input:$input){jobId traceId jobType state submittedAt message}}",
  "variables": {
    "input": {
      "jobType": "weather",
      "payload": {
        "city": "Austin",
        "country_code": "US",
        "units": "metric"
      }
    }
  }
}
```

Payload variants by `jobType`:

```json
{"city":"Austin","country_code":"US","units":"metric"}
```

```json
{"category":"inspirational","limit":1}
```

```json
{"base_currency":"USD","target_currency":"EUR"}
```

```json
{"username":"torvalds"}
```

Status query:

```json
{
  "query": "query JobStatus($jobId: ID!){jobStatus(jobId:$jobId){jobId state progressPercent message timestamp}}",
  "variables": {
    "jobId": "<job_id_from_submit>"
  }
}
```

curl submit example:

```bash
curl -s -X POST http://localhost:8080/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query":"mutation SubmitJob($input: SubmitJobInput!){submitJob(input:$input){jobId traceId jobType state submittedAt message}}",
    "variables":{"input":{"jobType":"weather","payload":{"city":"Austin","country_code":"US","units":"metric"}}}
  }'
```

## Teardown

```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down
```

With volume cleanup:

```bash
docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down -v
```
