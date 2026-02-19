# Week 3 AKS Manifests

Baseline Kubernetes manifests for Week 3 AKS deployment.

## Current Scope (Day 16)

- Base namespace
- Runtime config and secret wiring for API/worker workloads
- API deployment + service
- Worker deployment
- Worker gRPC service for API-to-worker calls
- UI deployment + service
- Kustomization entrypoint for image overrides
- Release deployment helper for shared image tag rollout
- Dry-run deploy-helper validation path for local readiness checks

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
