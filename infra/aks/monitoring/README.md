# Week 4 Monitoring Manifests (Day 17)

This folder contains AKS monitoring resources for Prometheus and Grafana.

## Scope

- Dedicated monitoring namespace (`dtq-monitoring`)
- Prometheus deployment + service
- Prometheus scrape config for:
  - API metrics (`dtq-api.dtq.svc.cluster.local:80/metrics`)
  - Worker metrics (`dtq-worker-metrics.dtq.svc.cluster.local:2112/metrics`)
- Grafana deployment + service

## Render

```bash
kubectl kustomize infra/aks/monitoring
```

## Apply

```bash
kubectl apply -k infra/aks/monitoring
```
