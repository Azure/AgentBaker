---
applyTo: "e2e/**"
---

# Reuse E2E scenarios

- Before adding a scenario, search existing scenarios for a compatible node and add the new checks there. Each separate scenario provisions another VM.
- Compare provisioning inputs, not just validators: OS image, architecture, VM size, cluster/network, NBC and ANC settings, VMSS tags, and provisioning mode.
- Preserve meaningful input coverage. When combining checks, account for interactions such as taints affecting pod scheduling, service restarts affecting later checks, and configuration changes affecting timing.
- Reuse validators and keep failures attributable to individual checks. Preserve required check order and restore state after disruptive checks.
- Add a separate scenario only when existing scenarios cannot cover the required inputs or safely run the checks. Explain that difference in the change description.
- Unit tests for scenario definitions are usually unnecessary. Focus unit tests on validator and framework behavior, rather than repeating scenario names, counts, or configuration values.

# E2E shared-environment rules

E2E scenarios run concurrently across branches and share predefined AKS clusters with unrelated tests.

E2E clusters also share infrastructure.

- Give each scenario-created Kubernetes object a unique name.
- Target each workload only to the scenario node.
- Do not modify or delete shared Kubernetes objects, other nodes, system pools, predefined clusters, or shared Azure infrastructure.
- Do not make cluster-wide changes in a scenario.
- Keep shared-infrastructure changes additive and idempotent.
- For shared-infrastructure changes, use new resource names to avoid collisions with concurrent branches.
- If a cluster configuration change can conflict across branches, change the configured cluster name to create a separate AKS cluster.
