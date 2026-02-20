# Week 3/4 AKS Manifests

Baseline Kubernetes manifests for Week 3 AKS deployment.

## Current Scope (Day 19)

- Base namespace
- Runtime config and secret wiring for API/worker workloads
- API deployment + service
- Worker deployment
- Worker gRPC service for API-to-worker calls
- Worker metrics service for Prometheus scraping
- UI deployment + service
- Kustomization entrypoint for image overrides
- Release deployment helper for shared image tag rollout
- Dry-run deploy-helper validation path for local readiness checks
- Monitoring kustomization for Prometheus and Grafana (`infra/aks/monitoring`)
- Worker reliability and shutdown-drain tuning via config map keys (`WORKER_STATUS_WRITE_*`, `WORKER_RESULT_WRITE_*`, `WORKER_SHUTDOWN_DRAIN_TIMEOUT`)

## Notes

- Base manifests intentionally use neutral `dtq-<component>:latest` images.
- CI/CD sets release images using one contract:
  - repository pattern: `${ACR_LOGIN_SERVER}/dtq-<component>`
  - shared tag: `${IMAGE_TAG}`
  - components: `api`, `worker`, `ui`
- Jenkins deploy stage delegates to `infra/aks/scripts/deploy-release.sh` so build/push/deploy all use the same image naming and tag strategy.
- `dtq-api-config` and `dtq-worker-config` are non-sensitive defaults and contract keys.
- `dtq-runtime-secrets` is a placeholder secret manifest and must be replaced with real values before cloud deployment.
- Redis/Mongo/Kafka are still out of this base set; manifests assume managed/cloud equivalents or separate workload manifests.
- Monitoring stack render path:
  - `kubectl kustomize infra/aks/monitoring`

## Render Example

```bash
kubectl kustomize infra/aks/base
```

## Release Deploy Helper

```bash
ACR_LOGIN_SERVER=myregistry.azurecr.io IMAGE_TAG=42 K8S_NAMESPACE=dtq \
  bash infra/aks/scripts/deploy-release.sh
```

Local dry-run validation (no cluster mutation):

```bash
ACR_LOGIN_SERVER=myregistry.azurecr.io IMAGE_TAG=42 K8S_NAMESPACE=dtq DRY_RUN=true \
  bash infra/aks/scripts/deploy-release.sh
```
