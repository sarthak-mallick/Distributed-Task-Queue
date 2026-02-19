# Week 3 AKS Manifests

Baseline Kubernetes manifests for Week 3 AKS deployment.

## Current Scope (Day 12)

- Base namespace
- API deployment + service
- Worker deployment
- UI deployment + service
- Kustomization entrypoint for image overrides

## Notes

- Images are placeholder defaults and must be overridden with ACR images during CI/CD.
- Redis/Mongo/Kafka are not deployed in this base set yet; Week 3 assumes managed/cloud equivalents or separate manifests.

## Render Example

```bash
kubectl kustomize infra/aks/base
```
