# Week 3 AKS Manifests

Baseline Kubernetes manifests for Week 3 AKS deployment.

## Current Scope (Day 13)

- Base namespace
- Runtime config and secret wiring for API/worker workloads
- API deployment + service
- Worker deployment
- Worker gRPC service for API-to-worker calls
- UI deployment + service
- Kustomization entrypoint for image overrides

## Notes

- Images are placeholder defaults and must be overridden with ACR images during CI/CD.
- `dtq-api-config` and `dtq-worker-config` are non-sensitive defaults and contract keys.
- `dtq-runtime-secrets` is a placeholder secret manifest and must be replaced with real values before cloud deployment.
- Redis/Mongo/Kafka are still out of this base set; manifests assume managed/cloud equivalents or separate workload manifests.

## Render Example

```bash
kubectl kustomize infra/aks/base
```
