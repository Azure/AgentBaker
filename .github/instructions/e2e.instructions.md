---
applyTo: "e2e/**"
---

# Reuse E2E scenarios

- When adding checks, defining a scenario, or renaming one, read [Writing and extending scenarios](../../e2e/README.md#writing-and-extending-scenarios). It defines the configuration-based naming and reuse pattern.
- Start by finding a scenario with the node settings the check needs. Extend its existing `Validator`; each new scenario creates separate test VM resources.
- Keep configuration, validators, and execution order together in the existing `Scenario`. Reuse is an authoring decision, not automatic grouping or a new compatibility-check mechanism.
- For a new scenario, state which required input or lifecycle prevents using an existing node.
- Before completing a rename, update in-repository selectors and documentation examples. Report the old and new names in the PR because external selectors and report history also use them.
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
