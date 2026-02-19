# Week 3 Execution Brief (Single Source of Truth)

Last updated: 2026-02-19  
Canonical product/timeline reference: `docs/project-spec.md` (Week 3 section + acceptance criteria)

This file is the active Week 3 execution and status artifact.

## Objective

Start and complete Week 3 Azure deployment foundations: Ansible-driven infra provisioning workflow, Kubernetes deployment manifests, and Jenkins CI/CD scaffolding for AKS deployment.

## Alignment Notes

- Week 2 migration is complete; Week 3 starts cloud deployment scope.
- Scope follows canonical Week 3 goals only:
  - Provision AKS, ACR, Jenkins VM through Ansible
  - Create Kubernetes manifests for all services
  - Build/push images and deploy through Jenkins pipeline
- Keep local Compose flow intact while introducing cloud artifacts.

## In Scope (Week 3 Only)

- Azure provisioning automation scaffolding (Ansible)
- AKS deployment manifest scaffolding (API, worker, UI, backing services where needed)
- Jenkins pipeline scaffolding (`build -> test -> push -> deploy`)
- Week 3 docs/runbook updates for cloud transition

## Out of Scope

- Prometheus/Grafana hardening work (Week 4)
- New feature scope in API/worker/UI behavior unrelated to deployment
- Production auth/tenancy expansions beyond current spec

## Ticket-Ready Task List

| ID | Task | Estimate | Depends On | Done When |
|----|------|----------|------------|-----------|
| W3-001 | Create Week 3 cloud infra scaffold (`infra/azure/ansible`, `infra/aks`, `Jenkinsfile`) | 0.5d | Week 2 complete | Directory structure and baseline config templates exist |
| W3-002 | Define Ansible variable contract and preflight checks for Azure provisioning | 0.5d | W3-001 | Required env/vars and validation playbook are documented and executable |
| W3-003 | Add baseline AKS Kubernetes manifests (namespace, api, worker, ui deployments/services) | 1d | W3-001 | Manifests render valid resource objects with configurable image tags |
| W3-004 | Add Jenkins pipeline scaffold for test/build/push/deploy stages | 1d | W3-001 | Pipeline file exists with stage placeholders and required env contracts |
| W3-005 | Integrate ACR image naming/tag strategy across manifests and pipeline | 0.5d | W3-003, W3-004 | Single image/tag contract shared across CI and manifests |
| W3-006 | Add Week 3 validation scripts/docs for cloud deployment handoff readiness | 0.5d | W3-002, W3-003, W3-004 | Runbook references and validation checklist are updated |

## Live Task Status

Update this table continuously during Week 3 execution.

| ID | Status | Last Updated | Notes |
|----|--------|--------------|-------|
| W3-001 | Completed | 2026-02-19 | Added Week 3 scaffold structure: `infra/azure/ansible`, `infra/aks`, `Jenkinsfile`, and active execution file. |
| W3-002 | Completed | 2026-02-19 | Added Azure variable contract template + Day 12 preflight validation playbook. |
| W3-003 | Completed | 2026-02-19 | Added Day 13 AKS manifest baseline depth: config/secret wiring, worker gRPC service, and deployment env/probe contract checks. |
| W3-004 | Completed | 2026-02-19 | Replaced Jenkins TODO placeholders with executable contract validation, Docker build/push steps, and AKS deploy/rollout commands. |
| W3-005 | Pending | 2026-02-19 | Planned for mid-week CI/CD wiring. |
| W3-006 | In Progress | 2026-02-19 | Integrated Day 14 Jenkins/Dockerfile checks into canonical `scripts/run-current-e2e.sh` and aligned README runbook. |

## Implementation Order

1. Start scaffolding first (`W3-001`).
2. Lock provisioning contracts and preflight checks (`W3-002`).
3. Add AKS manifests (`W3-003`).
4. Add Jenkins pipeline (`W3-004`).
5. Align image/tag contracts (`W3-005`).
6. Final validation/runbook updates (`W3-006`).

## Day-by-Day Plan

### Day 12 - Week 3 Kickoff
- Start W3-001 and W3-002.
- Create Azure/AKS/Jenkins scaffolding and provisioning variable contract.

### Day 13 - AKS Manifest Baseline
- Execute W3-003.
- Validate manifest structure and environment variable wiring.

### Day 14 - Jenkins Pipeline Baseline
- Execute W3-004.
- Wire test/build/push/deploy stage skeleton.

### Day 15 - CI/CD and Manifest Contract Alignment
- Execute W3-005.
- Standardize image registry/tag flow from pipeline to manifests.

### Day 16 - Validation + Runbook
- Execute W3-006.
- Confirm Week 3 readiness checklist and handoff docs.

## Week 3 Acceptance Checklist

- Infra provisioning workflow artifacts exist and are documented.
- Kubernetes manifests for all services exist with configurable images.
- Jenkins pipeline stages support build/test/push/deploy flow.
- Week 2 functional path remains intact while cloud artifacts are introduced.

## Session Log (Append-Only)

- 2026-02-19: Week 3 execution file created; Day 12 kickoff started.
- 2026-02-19: Completed Day 12 infrastructure scaffold (`W3-001`) and provisioning preflight contract (`W3-002`).
- 2026-02-19: Started batched early drafts for AKS manifests (`W3-003`) and Jenkins pipeline scaffold (`W3-004`).
- 2026-02-19: Validation check results: `kubectl kustomize infra/aks/base` passed; Ansible syntax check skipped locally (`ansible-playbook` unavailable).
- 2026-02-19: Updated canonical `scripts/run-current-e2e.sh` to include Week 3 Day 12 scaffold checks (Jenkinsfile, AKS kustomize, Ansible preflight when available); updated README accordingly.
- 2026-02-19: Revalidated canonical runner after Week 3 check integration (`API_PORT=28080 WORKER_GRPC_ADDR=127.0.0.1:29090 bash scripts/run-current-e2e.sh --purge`) with all runtime checks passing.
- 2026-02-19: Completed Day 13 AKS manifest baseline (`W3-003`) with `dtq-api-config`, `dtq-worker-config`, `dtq-runtime-secrets`, `dtq-worker-grpc`, and API/worker env wiring updates.
- 2026-02-19: Updated canonical `scripts/run-current-e2e.sh` and `README.md` for Day 13 validation coverage (AKS render + manifest/env wiring assertions + Week 3 preflight).
- 2026-02-19: Revalidated Day 13 cumulative flow with UI checks (`API_PORT=28080 WORKER_GRPC_ADDR=127.0.0.1:29090 bash scripts/run-current-e2e.sh --with-ui-checks --purge`) and all checks passed.
- 2026-02-19: Completed Day 14 Jenkins pipeline baseline (`W3-004`) with executable CI contract validation, Docker build/push, and AKS deployment rollout steps in `Jenkinsfile`.
- 2026-02-19: Added Day 14 containerization artifacts (`api/worker/ui` Dockerfiles and `ui/nginx.conf`) required by Jenkins build stages.
- 2026-02-19: Updated canonical `scripts/run-current-e2e.sh` + `README.md` to Day 14 checks and revalidated full flow (`API_PORT=28080 WORKER_GRPC_ADDR=127.0.0.1:29090 bash scripts/run-current-e2e.sh --with-ui-checks --purge`) with all checks passing.
- 2026-02-19: Validated Day 14 container builds locally (`docker build` for `api`, `worker`, and `ui`) with all three images building successfully.

## Handoff Snapshot

- Week status: In progress (Day 14 completed)
- Completed tasks: W3-001, W3-002, W3-003, W3-004
- In-progress tasks: W3-006
- Blockers: None
- Next task: Execute Day 15 (`W3-005`) to standardize image registry/tag contract across Jenkins pipeline and AKS manifests.
