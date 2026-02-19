# Week 3 Azure Ansible Scaffold

This directory contains Week 3 Ansible scaffolding for Azure provisioning.

## Current Scope (Day 12)

- Baseline Ansible config and collection requirements
- Variable contract template for Azure resources
- Preflight playbook to validate required variables before provisioning

## Layout

- `ansible.cfg` - local Ansible defaults for this folder
- `requirements.yml` - required collections (Azure)
- `inventories/dev/group_vars/all.yml.example` - required variables template
- `playbooks/day12-preflight.yml` - non-destructive validation playbook

## Usage

1. Install Ansible collections:

```bash
cd infra/azure/ansible
ansible-galaxy collection install -r requirements.yml
```

2. Copy variable template:

```bash
cp inventories/dev/group_vars/all.yml.example inventories/dev/group_vars/all.yml
```

3. Fill values in `inventories/dev/group_vars/all.yml`.

4. Run preflight:

```bash
ansible-playbook -i localhost, playbooks/day12-preflight.yml \
  -e @inventories/dev/group_vars/all.yml
```

The preflight playbook performs validation only and does not provision resources.
