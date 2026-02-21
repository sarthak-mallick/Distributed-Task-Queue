# Azure Runbook

Last updated: 2026-02-21

Use this guide to deploy and smoke-test the project on Azure AKS.

## Prerequisites

- Azure subscription with permissions for ACR + AKS
- `az`, `kubectl`, `docker`
- Repository cloned and local Docker working

Optional preflight scaffold:
- `ansible-playbook` for `infra/azure/ansible/playbooks/day12-preflight.yml`

## 1) Validate Deployment Contracts Locally

```bash
kubectl kustomize infra/aks/base
kubectl kustomize infra/aks/monitoring
ACR_LOGIN_SERVER=example.azurecr.io IMAGE_TAG=contract-check K8S_NAMESPACE=dtq DRY_RUN=true \
  bash infra/aks/scripts/deploy-release.sh
```

## 2) Prepare Runtime Config and Secrets

Set real values for cloud dependencies before deployment:
- `infra/aks/base/api-configmap.yaml`
- `infra/aks/base/worker-configmap.yaml`
- `infra/aks/base/runtime-secrets.yaml`

At minimum, replace placeholder values for Kafka, Redis, RabbitMQ, and Mongo endpoints/credentials.

## 3) Build and Push Images to ACR

```bash
export ACR_LOGIN_SERVER="<your-registry>.azurecr.io"
export IMAGE_TAG="manual-$(date +%Y%m%d%H%M%S)"

az acr login --name "${ACR_LOGIN_SERVER%%.azurecr.io}"

for component in api worker ui; do
  image_ref="${ACR_LOGIN_SERVER}/dtq-${component}:${IMAGE_TAG}"
  docker build -f "${component}/Dockerfile" -t "${image_ref}" "${component}"
  docker push "${image_ref}"
done
```

## 4) Deploy to AKS

```bash
export AZURE_SUBSCRIPTION_ID="<subscription-id>"
export AZURE_AKS_RESOURCE_GROUP="<aks-resource-group>"
export AZURE_AKS_CLUSTER_NAME="<aks-cluster-name>"
export K8S_NAMESPACE="dtq"

az account set --subscription "${AZURE_SUBSCRIPTION_ID}"
az aks get-credentials \
  --resource-group "${AZURE_AKS_RESOURCE_GROUP}" \
  --name "${AZURE_AKS_CLUSTER_NAME}" \
  --overwrite-existing

ACR_LOGIN_SERVER="${ACR_LOGIN_SERVER}" IMAGE_TAG="${IMAGE_TAG}" K8S_NAMESPACE="${K8S_NAMESPACE}" \
  bash infra/aks/scripts/deploy-release.sh

kubectl apply -k infra/aks/monitoring
```

## 5) Smoke Test on Cluster

1. Port-forward API:

```bash
kubectl -n dtq port-forward svc/dtq-api 8080:80
```

2. Submit/query using GraphQL payloads from `docs/runbooks/local-runbook.md`.

3. Verify UI service:

```bash
kubectl -n dtq get svc dtq-ui
```

Open the `EXTERNAL-IP` URL and point the UI API base URL to the port-forwarded API when testing from your browser.

4. Verify monitoring:

```bash
kubectl -n dtq-monitoring port-forward svc/dtq-prometheus 9090:9090
kubectl -n dtq-monitoring port-forward svc/dtq-grafana 3000:3000
```

Open [http://localhost:3000](http://localhost:3000) and confirm dashboard `DTQ Operational Overview`.

## 6) Optional Ansible Preflight

```bash
cd infra/azure/ansible
cp inventories/dev/group_vars/all.yml.example inventories/dev/group_vars/all.yml
# fill values in all.yml
ansible-playbook -i localhost, playbooks/day12-preflight.yml \
  -e @inventories/dev/group_vars/all.yml
```
