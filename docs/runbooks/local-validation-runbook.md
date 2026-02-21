# Local Validation Runbook

Last updated: 2026-02-21

Use this guide to validate the latest completed flow locally.

## Canonical Validation Command

From repository root:

```bash
bash scripts/run-current-e2e.sh --with-ui-checks --purge
```

This validates:
- GraphQL mutation/query/subscription runtime flow
- API-worker gRPC status/progress path
- Redis/Mongo/Kafka data-path checks
- API/worker unit tests
- UI build checks (with `--with-ui-checks`)
- Week 4 Day 20 Jenkins/AKS/monitoring scaffold checks
- Local image build checks for `api`, `worker`, `ui`

## Common Variants

Backend-only validation:

```bash
bash scripts/run-current-e2e.sh
```

Keep infra running after test:

```bash
bash scripts/run-current-e2e.sh --keep-infra
```

Skip unit tests:

```bash
bash scripts/run-current-e2e.sh --skip-unit-tests
```

Skip image build checks:

```bash
bash scripts/run-current-e2e.sh --skip-image-build-checks
```

Skip Week 4 scaffold checks:

```bash
bash scripts/run-current-e2e.sh --skip-week4-checks
```

## Manual Spot-Checks (Optional)

- API health: `curl -s http://localhost:8080/healthz`
- API metrics: `curl -s http://localhost:8080/metrics`
- Worker metrics: `curl -s http://localhost:2112/metrics`
- UI HTTP check (if running): `curl -i http://localhost:5173/`
