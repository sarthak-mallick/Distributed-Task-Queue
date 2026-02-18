# UI Service (Week 2)

React UI with Apollo Client for Week 2 GraphQL migration.

Current UI behavior:
- submit jobs via GraphQL mutation (`submitJob`)
- query current status via GraphQL query (`jobStatus`)
- stream live progress via GraphQL subscription (`jobProgress`) over WebSocket

## Run Locally

```bash
cd ui
npm install
npm run dev
```

Open `http://localhost:5173`.

For one-command backend + frontend validation from repo root:

```bash
bash scripts/run-current-e2e.sh --with-ui-checks
```

## Manual Validation Path

1. Start backend dependencies and services from repo root:
   - `bash infra/compose/scripts/bootstrap-and-smoke.sh`
   - Terminal 1: `cd worker && go run .`
   - Terminal 2: `cd api && go run .`
2. In a separate terminal, run the UI (`npm run dev`).
3. In the browser:
   - submit a `weather` job
   - confirm a `jobId` is returned
   - confirm progress updates stream live and reach `completed` with `100%`
4. Teardown when done:
   - `docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down`
