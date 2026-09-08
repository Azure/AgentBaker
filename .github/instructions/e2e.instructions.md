---
applyTo: "e2e/**"
---

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
