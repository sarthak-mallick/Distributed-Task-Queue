# Week 4 Monitoring Manifests (Day 20)

This folder contains AKS monitoring resources for Prometheus and Grafana, including alert rules and pre-provisioned dashboards for Week 4 acceptance closure.

## Scope

- Dedicated monitoring namespace (`dtq-monitoring`)
- Prometheus deployment + service
- Prometheus alert rules config map:
  - `DTQAPIGraphQLFailureRateHigh`
  - `DTQWorkerJobFailuresDetected`
  - `DTQWorkerShutdownDrainTimeoutDetected`
- Prometheus scrape config for:
  - API metrics (`dtq-api.dtq.svc.cluster.local:80/metrics`)
  - Worker metrics (`dtq-worker-metrics.dtq.svc.cluster.local:2112/metrics`)
- Grafana deployment + service
- Grafana datasource provisioning (`DTQ Prometheus`)
- Grafana dashboard provisioning (`DTQ Operational Overview`)

## Render

```bash
kubectl kustomize infra/aks/monitoring
```

## Apply

```bash
kubectl apply -k infra/aks/monitoring
```

## Local Verification (AKS Cluster Context)

Prometheus:

```bash
kubectl -n dtq-monitoring port-forward svc/dtq-prometheus 9090:9090
```

Grafana:

```bash
kubectl -n dtq-monitoring port-forward svc/dtq-grafana 3000:3000
```

Then:
- Open `http://localhost:3000` (default `admin` / `admin`)
- Confirm dashboard `DTQ Operational Overview` is present in folder `DTQ`
- Confirm Prometheus alerts page lists the `DTQ*` alert rules
