# UI Service (Week 1)

Basic React UI for:
- submitting generic jobs (`weather`, `quote`, `exchange_rate`, `github_user`)
- checking job status/progress by `job_id`
- polling live progress from `GET /v1/jobs/{job_id}/status`

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
   - Terminal 1: `cd api && go run .`
   - Terminal 2: `cd worker && go run .`
2. In a separate terminal, run the UI (`npm run dev`).
3. In the browser:
   - submit a `weather` job from the form
   - confirm a `job_id` is returned
   - confirm progress reaches `completed` with `100%`
4. Teardown when done:
   - `docker compose -f infra/compose/docker-compose.yml --env-file infra/compose/.env down`
