#!/usr/bin/env bash
set -euo pipefail

# deploy-release applies base manifests and updates deployments to one release tag.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
AKS_BASE_DIR="${ROOT_DIR}/infra/aks/base"

K8S_NAMESPACE="${K8S_NAMESPACE:-dtq}"
ACR_LOGIN_SERVER="${ACR_LOGIN_SERVER:-}"
IMAGE_TAG="${IMAGE_TAG:-}"

# log writes stable status lines for CI output scanning.
log() {
  printf '[aks-deploy] %s\n' "$1"
}

# require_env ensures one required variable is present.
require_env() {
  local key="$1"
  if [[ -z "${!key:-}" ]]; then
    log "missing required environment variable: ${key}"
    exit 1
  fi
}

# image_ref returns the canonical release image reference for one component.
image_ref() {
  local component="$1"
  printf '%s/dtq-%s:%s' "${ACR_LOGIN_SERVER}" "${component}" "${IMAGE_TAG}"
}

require_env "ACR_LOGIN_SERVER"
require_env "IMAGE_TAG"

if [[ ! -f "${AKS_BASE_DIR}/kustomization.yaml" ]]; then
  log "missing AKS kustomization at ${AKS_BASE_DIR}/kustomization.yaml"
  exit 1
fi

log "applying AKS base manifests"
kubectl apply -k "${AKS_BASE_DIR}"

log "setting release images with shared tag=${IMAGE_TAG}"
kubectl -n "${K8S_NAMESPACE}" set image deployment/dtq-api api="$(image_ref api)"
kubectl -n "${K8S_NAMESPACE}" set image deployment/dtq-worker worker="$(image_ref worker)"
kubectl -n "${K8S_NAMESPACE}" set image deployment/dtq-ui ui="$(image_ref ui)"

log "waiting for deployment rollouts"
kubectl -n "${K8S_NAMESPACE}" rollout status deployment/dtq-api --timeout=300s
kubectl -n "${K8S_NAMESPACE}" rollout status deployment/dtq-worker --timeout=300s
kubectl -n "${K8S_NAMESPACE}" rollout status deployment/dtq-ui --timeout=300s

log "deployment rollout complete"
kubectl -n "${K8S_NAMESPACE}" get deployments
