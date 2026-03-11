# Scriptless vs Non-Scriptless Architecture Comparison

## High-Level Overview

### Non-Scriptless (Traditional/Legacy)
- **Heavy dependency** on versioned AgentBaker service pods (26+ deployments)
- **Large CustomData** payloads (~100KB with embedded scripts)
- **Tight coupling** between control plane and node bootstrap code

### Scriptless (New Architecture)
- **Minimal dependency** on AgentBaker services (1 service for VHD metadata only)
- **Small CustomData** payloads (~5-10KB with just config JSON)
- **Loose coupling** via Protobuf contract for forward/backward compatibility

---

## Architecture Diagram: Non-Scriptless Path

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   AZURE BACKEND INFRASTRUCTURE                              │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │  AgentBaker Service Pods (26+ Deployments)                            │ │
│  │                                                                        │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │ │
│  │  │ agentbaker-svc  │  │ agentbaker-svc  │  │ agentbaker-svc  │      │ │
│  │  │   v1.27.0       │  │   v1.27.1       │  │   v1.28.0       │      │ │
│  │  │                 │  │                 │  │                 │      │ │
│  │  │ API:            │  │ API:            │  │ API:            │      │ │
│  │  │ /getnode        │  │ /getnode        │  │ /getnode        │      │ │
│  │  │ bootstrap       │  │ bootstrap       │  │ bootstrap       │      │ │
│  │  │ data            │  │ data            │  │ data            │      │ │
│  │  │                 │  │                 │  │                 │      │ │
│  │  │ Code:           │  │ Code:           │  │ Code:           │      │ │
│  │  │ pkg/agent/      │  │ pkg/agent/      │  │ pkg/agent/      │      │ │
│  │  │ baker.go        │  │ baker.go        │  │ baker.go        │      │ │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────┘      │ │
│  │                                                                        │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │ │
│  │  │ agentbaker-svc  │  │ agentbaker-svc  │  │ agentbaker-svc  │      │ │
│  │  │   v1.28.5       │  │   v1.29.0       │  │   v1.29.1       │      │ │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────┘      │ │
│  │                                                                        │ │
│  │  ... (26+ total pods, one per K8s version + patches)                  │ │
│  │                                                                        │ │
│  └───────────────────────────────┬────────────────────────────────────────┘ │
│                                  │                                          │
└──────────────────────────────────┼──────────────────────────────────────────┘
                                   │
                                   │ HTTP POST /getnodebootstrapdata
                                   │ Body: NodeBootstrappingConfiguration (JSON)
                                   │ {
                                   │   "kubernetesVersion": "1.29.0",
                                   │   "agentPoolProfile": {...},
                                   │   "localDnsProfile": {
                                   │     "enableHostsPlugin": true
                                   │   }
                                   │ }
                                   │
┌──────────────────────────────────▼──────────────────────────────────────────┐
│  AKS Resource Provider (AKS-RP) - Control Plane                            │
│                                                                             │
│  User Request: Create node pool with K8s 1.29.0                            │
│                                                                             │
│  Step 1: Determine which AgentBaker service to call                        │
│    └─> K8s version 1.29.0 → Call agentbaker-svc-v1.29.0                   │
│                                                                             │
│  Step 2: POST NodeBootstrappingConfiguration                               │
│    └─> AgentBaker service generates CSE + CustomData                       │
│                                                                             │
│  Step 3: Receive response                                                  │
│    ├─> CSE: "/bin/bash /opt/azure/containers/provision.sh"                │
│    └─> CustomData: ~100KB YAML with embedded bash scripts                 │
│                                                                             │
└──────────────────────────────────┬─────────────────────────────────────────┘
                                   │
                                   │ ARM API Call
                                   │ Create VMSS with CustomData + CSE
                                   │
┌──────────────────────────────────▼─────────────────────────────────────────┐
│  Azure Resource Manager (ARM)                                              │
│                                                                             │
│  Creates VM with:                                                           │
│  - CustomData: Large YAML (~100KB)                                         │
│  - CSE: Bash command to execute                                            │
│                                                                             │
└──────────────────────────────────┬─────────────────────────────────────────┘
                                   │
                                   │ VM Provisioning
                                   │
┌──────────────────────────────────▼─────────────────────────────────────────┐
│  Virtual Machine (Node)                                                     │
│                                                                             │
│  CustomData delivered (~100KB):                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ #cloud-config                                                        │  │
│  │ write_files:                                                         │  │
│  │ - path: /opt/azure/containers/provision.sh                          │  │
│  │   content: !!binary | <base64 encoded bash script>                  │  │
│  │ - path: /opt/azure/containers/provision_source.sh                   │  │
│  │   content: !!binary | <base64 encoded bash script>                  │  │
│  │ - path: /opt/azure/containers/cse_main.sh                           │  │
│  │   content: !!binary | <base64 encoded bash script>                  │  │
│  │ - path: /opt/azure/containers/cse_config.sh                         │  │
│  │   content: !!binary | <base64 encoded bash script>                  │  │
│  │ - path: /opt/azure/containers/cse_helpers.sh                        │  │
│  │   content: !!binary | <base64 encoded bash script>                  │  │
│  │ - path: /opt/azure/containers/cse_cmd.sh                            │  │
│  │   content: |                                                         │  │
│  │     SHOULD_ENABLE_HOSTS_PLUGIN="true"                                │  │
│  │     LOCALDNS_GENERATED_COREFILE="<base64>"                           │  │
│  │     ... (all templated variables)                                    │  │
│  │ ... (many more scripts)                                              │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  Step 1: cloud-init processes CustomData                                   │
│    └─> Writes all scripts to disk                                          │
│                                                                             │
│  Step 2: CSE executes                                                       │
│    └─> /bin/bash /opt/azure/containers/provision.sh                        │
│        ├─> Sources cse_cmd.sh (variables)                                  │
│        ├─> Executes cse_main.sh                                            │
│        │   ├─> enableAKSHostsSetup()                                       │
│        │   └─> select_localdns_corefile()                                  │
│        └─> Node bootstraps and joins cluster                               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Characteristics:**
- 📦 **CustomData size:** ~100KB (all scripts embedded)
- 🔄 **Scripts uploaded:** Every provisioning
- 🌐 **Network dependency:** Requires AgentBaker service call
- 🔧 **Maintenance burden:** 26+ service pods to maintain
- 📝 **Configuration:** Go template replacement in baker.go

---

## Architecture Diagram: Scriptless Path

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                   AZURE BACKEND INFRASTRUCTURE                              │
│                                                                             │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │  AgentBaker Service (Simplified to 1 pod)                             │ │
│  │                                                                        │ │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │ │
│  │  │ agentbaker-sig-image-service                                    │  │ │
│  │  │                                                                 │  │ │
│  │  │ API: /getlatestsigimageconfig                                  │  │ │
│  │  │ API: /getdistrosigimageconfig                                  │  │ │
│  │  │                                                                 │  │ │
│  │  │ Purpose: Provide latest VHD image versions for each OS/K8s     │  │ │
│  │  │                                                                 │  │ │
│  │  │ Example Response:                                               │  │ │
│  │  │ {                                                               │  │ │
│  │  │   "ubuntu2204": {                                               │  │ │
│  │  │     "2025.02.10": {                                             │  │ │
│  │  │       "sigImageVersionId": "...",                               │  │ │
│  │  │       "publishDate": "2025-02-10"                               │  │ │
│  │  │     }                                                            │  │ │
│  │  │   }                                                              │  │ │
│  │  │ }                                                                │  │ │
│  │  └─────────────────────────────────────────────────────────────────┘  │ │
│  │                                                                        │ │
│  │  NO versioned pods needed! Just metadata service. ✅                   │ │
│  │                                                                        │ │
│  └────────────────────────────────┬───────────────────────────────────────┘ │
│                                   │                                         │
└───────────────────────────────────┼─────────────────────────────────────────┘
                                    │
                                    │ (Optional) GET /getlatestsigimageconfig
                                    │ Returns: Latest VHD image versions
                                    │
┌───────────────────────────────────▼─────────────────────────────────────────┐
│  AKS Resource Provider (AKS-RP) - Control Plane                            │
│                                                                             │
│  User Request: Create node pool with K8s 1.29.0                            │
│                                                                             │
│  Step 1: (Optional) Query latest VHD image for Ubuntu 22.04                │
│    └─> Call /getlatestsigimageconfig once (not per node!)                 │
│                                                                             │
│  Step 2: Generate AKSNodeConfig CLIENT-SIDE (no service call!)             │
│    ├─> Use aks-node-controller/proto schema                                │
│    └─> Protobuf JSON (~5KB):                                               │
│        {                                                                    │
│          "version": "v1",                                                   │
│          "kubernetesVersion": "1.29.0",                                     │
│          "localDnsProfile": {                                               │
│            "enableLocalDns": true,                                          │
│            "enableHostsPlugin": true                                        │
│          },                                                                 │
│          "kubeletConfig": {...},                                            │
│          "customLinuxOsConfig": {...}                                       │
│        }                                                                    │
│                                                                             │
│  Step 3: Generate CustomData and CSE CLIENT-SIDE                           │
│    ├─> CustomData: Base64 encode AKSNodeConfig JSON (~5KB)                 │
│    └─> CSE: "/opt/azure/containers/aks-node-controller provision-wait"     │
│                                                                             │
│  Step 4: Send directly to ARM (no AgentBaker call per node!)               │
│                                                                             │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │
                                   │ ARM API Call
                                   │ Create VMSS with CustomData + CSE
                                   │
┌──────────────────────────────────▼──────────────────────────────────────────┐
│  Azure Resource Manager (ARM)                                               │
│                                                                              │
│  Creates VM with:                                                            │
│  - CustomData: Small YAML (~5-10KB) with config JSON                        │
│  - CSE: /opt/azure/containers/aks-node-controller provision-wait            │
│                                                                              │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │
                                   │ VM Provisioning
                                   │
┌──────────────────────────────────▼──────────────────────────────────────────┐
│  Virtual Machine (Node) - NEW VHD                                           │
│                                                                              │
│  VHD already contains (baked in during VHD build):                          │
│  ├─> /opt/azure/containers/aks-node-controller (Go binary)                 │
│  ├─> /opt/azure/containers/cse_main.sh                                     │
│  ├─> /opt/azure/containers/cse_config.sh                                   │
│  ├─> /opt/azure/containers/cse_helpers.sh                                  │
│  ├─> /opt/azure/containers/aks-hosts-setup.sh                              │
│  ├─> /etc/systemd/system/aks-node-controller.service                       │
│  └─> /etc/systemd/system/aks-hosts-setup.timer                             │
│                                                                              │
│  CustomData delivered (~5KB):                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │ #cloud-config                                                        │  │
│  │ write_files:                                                         │  │
│  │ - path: /opt/azure/containers/aks-node-controller-config.json       │  │
│  │   permissions: "0755"                                                │  │
│  │   owner: root                                                        │  │
│  │   content: !!binary |                                                │  │
│  │     ewogICJ2ZXJzaW9uIjogInYxIiwKICAia3ViZXJuZXRlc1ZlcnNpb24iOiAi │  │
│  │     MS4yOS4wIiwKICAibG9jYWxEbnNQcm9maWxlIjogewogICAgImVuYWJsZU │  │
│  │     ... (Just base64 encoded JSON config, ~5KB total)                │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │                    PROVISIONING SEQUENCE                               │ │
│  ├────────────────────────────────────────────────────────────────────────┤ │
│  │                                                                        │ │
│  │  1. cloud-init processes CustomData                                   │ │
│  │     └─> Writes /opt/azure/containers/aks-node-controller-config.json │ │
│  │                                                                        │ │
│  │  2. cloud-init completes → Triggers systemd                           │ │
│  │     └─> Starts aks-node-controller.service                            │ │
│  │         (After=cloud-init.target)                                     │ │
│  │                                                                        │ │
│  │  3. aks-node-controller.service executes                              │ │
│  │     ExecStart=/opt/azure/containers/aks-node-controller provision     │ │
│  │     │                                                                  │ │
│  │     ├─> Read /opt/azure/containers/aks-node-controller-config.json   │ │
│  │     ├─> Parse Protobuf (tolerant - ignores unknown fields)            │ │
│  │     ├─> Convert to environment variables via parser                   │ │
│  │     │   (aks-node-controller/parser/parser.go)                        │ │
│  │     │                                                                  │ │
│  │     ├─> Generate env vars:                                             │ │
│  │     │   SHOULD_ENABLE_HOSTS_PLUGIN="true"                             │ │
│  │     │   LOCALDNS_GENERATED_COREFILE="<base64>"                        │ │
│  │     │   LOCALDNS_GENERATED_COREFILE_NO_HOSTS="<base64>"              │ │
│  │     │                                                                  │ │
│  │     ├─> Execute existing bash scripts with env vars:                  │ │
│  │     │   ├─> Source environment variables                              │ │
│  │     │   ├─> /opt/azure/containers/cse_main.sh                        │ │
│  │     │   │   ├─> enableAKSHostsSetup()                                │ │
│  │     │   │   └─> select_localdns_corefile()                           │ │
│  │     │   └─> Node bootstrap completes                                 │ │
│  │     │                                                                  │ │
│  │     └─> Write /opt/azure/containers/provision.complete                │ │
│  │                                                                        │ │
│  │  4. CSE executes (in parallel with step 3)                            │ │
│  │     /opt/azure/containers/aks-node-controller provision-wait          │ │
│  │     │                                                                  │ │
│  │     ├─> Poll for /opt/azure/containers/provision.complete            │ │
│  │     ├─> Read /var/log/azure/aks/provision.json (status)              │ │
│  │     └─> Return to ARM via stdout                                      │ │
│  │                                                                        │ │
│  │  5. Node joins cluster                                                 │ │
│  │                                                                        │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Key Characteristics:**
- 📦 **CustomData size:** ~5-10KB (just config JSON)
- 🔄 **Scripts location:** Pre-installed on VHD (not uploaded per node)
- 🌐 **Network dependency:** Minimal (optional VHD version query)
- 🔧 **Maintenance burden:** 1 service pod (or zero if client-side only)
- 📝 **Configuration:** Protobuf contract + parser

---

## Side-by-Side Comparison

| Aspect | 🔴 Non-Scriptless (Legacy) | 🟢 Scriptless (New) |
|--------|---------------------------|---------------------|
| **AgentBaker Service Pods** | 26+ versioned pods (one per K8s version) | 1 pod (VHD metadata only) or 0 (fully client-side) |
| **Service Call per Node** | ✅ YES - Every node provisioning calls AgentBaker | ❌ NO - Only occasional VHD version queries |
| **CustomData Size** | ~100KB (embedded bash scripts) | ~5-10KB (just JSON config) |
| **Scripts Location** | Uploaded per node via CustomData | Pre-installed on VHD during build |
| **CSE Command** | `/bin/bash /opt/azure/containers/provision.sh` | `/opt/azure/containers/aks-node-controller provision-wait` |
| **Configuration Format** | `NodeBootstrappingConfiguration` (Go struct) | `AKSNodeConfig` (Protobuf) |
| **Config Source** | `pkg/agent/datamodel/types.go` | `aks-node-controller/proto/aksnodeconfig/v1/*.proto` |
| **Template Engine** | Go text/template in `pkg/agent/baker.go` | aks-node-controller parser + TOML templates |
| **Variable Injection** | Direct template replacement: `{{ShouldEnableHostsPlugin}}` | Protobuf → Parser → Environment variables |
| **Bootstrap Orchestrator** | Bash scripts (cse_main.sh) | Go binary (aks-node-controller) → Bash scripts |
| **Forward Compatibility** | ❌ NO - Must match K8s version exactly | ✅ YES - Protobuf tolerant parsing ignores unknown fields |
| **Backward Compatibility** | ❌ NO - Old VHDs can't use new CustomData format | ✅ YES - aks-node-controller can parse older config versions |
| **Network Efficiency** | ❌ Poor - Large payloads, many service calls | ✅ Good - Small payloads, minimal service calls |
| **Scalability** | ❌ Poor - 1000 nodes = 1000 AgentBaker calls | ✅ Excellent - 1000 nodes = 0-1 AgentBaker calls |
| **Deployment Complexity** | ❌ High - Deploy/maintain 26+ service versions | ✅ Low - Deploy 1 service or none |

---

## The 26+ Pods Explained

### Why Exactly 26+?

**Calculation:**
```
Supported Kubernetes versions:
  - Last 3-4 minor versions (e.g., 1.27, 1.28, 1.29, 1.30)
  - Multiple patches per minor version (e.g., 1.29.0, 1.29.1, 1.29.2, 1.29.3...)

Example (simplified):
  K8s 1.27: 7 patch versions → 7 pods
  K8s 1.28: 8 patch versions → 8 pods
  K8s 1.29: 6 patch versions → 6 pods
  K8s 1.30: 5 patch versions → 5 pods
  ────────────────────────────────────
  Total:                      26 pods

Actual number varies as:
  - New K8s versions released
  - Old versions sunset
  - Security patches released
```

### What Each Pod Does

Each pod runs the **same Go code** (`apiserver/*.go` + `pkg/agent/baker.go`) but is configured for a specific Kubernetes version:

```go
// Each pod internally does this:
agentBaker.GetNodeBootstrapping(ctx, &config)
  ├─> config.KubernetesVersion = "1.29.0" (for agentbaker-v1.29.0 pod)
  ├─> Generate kubelet flags for 1.29.0
  ├─> Generate component versions compatible with 1.29.0
  ├─> Template bash scripts with 1.29.0-specific values
  └─> Return CustomData + CSE
```

### The Deployment Architecture (Non-Scriptless)

```
┌───────────────────────────────────────────────────────────────────┐
│  Kubernetes Cluster (Azure Backend)                              │
│                                                                   │
│  Namespace: agentbaker-system (example)                           │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  Deployment: agentbaker-v1-27-0                             │ │
│  │  Replicas: 3-5 (for HA)                                     │ │
│  │  Container: agentbaker:v1.27.0                              │ │
│  │  Env: KUBERNETES_VERSION=1.27.0                             │ │
│  │  Service: agentbaker-v1-27-0-svc                            │ │
│  │    └─> ClusterIP: 10.0.1.100:8080                           │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  Deployment: agentbaker-v1-27-1                             │ │
│  │  Replicas: 3-5                                              │ │
│  │  Container: agentbaker:v1.27.1                              │ │
│  │  Env: KUBERNETES_VERSION=1.27.1                             │ │
│  │  Service: agentbaker-v1-27-1-svc                            │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  ... (repeat for all 26+ K8s versions)                           │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  Deployment: agentbaker-v1-30-5                             │ │
│  │  Replicas: 3-5                                              │ │
│  │  Container: agentbaker:v1.30.5                              │ │
│  │  Service: agentbaker-v1-30-5-svc                            │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │  Load Balancer / Ingress                                    │ │
│  │  Routes based on K8s version in request                     │ │
│  │    /getnodebootstrapdata?version=1.29.0 → agentbaker-v1-29-0│ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
```

**Deployment Characteristics:**
- Each deployment: 3-5 replicas for HA
- Total pods: 26 deployments × 3-5 replicas = **78-130 running pods**
- Resource overhead: Each pod needs memory, CPU, updates, monitoring
- Deployment complexity: Each K8s release requires new AgentBaker deployment

---

## Detailed Flow Comparison

### Non-Scriptless: Request Flow for 1000 Nodes

```
User: Scale node pool to 1000 nodes (K8s 1.29.0)
  │
  ├─> AKS-RP receives request
  │
  ├─> For each of 1000 nodes:
  │   │
  │   ├─> HTTP POST to agentbaker-v1-29-0-svc
  │   │   URL: /getnodebootstrapdata
  │   │   Body: NodeBootstrappingConfiguration (JSON, ~2KB)
  │   │   {
  │   │     "kubernetesVersion": "1.29.0",
  │   │     "agentPoolProfile": {...},
  │   │     "localDnsProfile": {"enableHostsPlugin": true}
  │   │   }
  │   │
  │   ├─> AgentBaker pod processes request:
  │   │   ├─> baker.GetNodeBootstrappingCmd()
  │   │   ├─> Template all bash scripts with values
  │   │   ├─> Generate large CustomData (~100KB)
  │   │   └─> Return CSE + CustomData
  │   │
  │   ├─> AKS-RP receives response (~100KB)
  │   │
  │   └─> AKS-RP sends to ARM for VM creation
  │
  └─> Result:
      - 1000 HTTP calls to AgentBaker
      - ~100MB traffic (1000 × 100KB)
      - AgentBaker pods under heavy load
      - Each CustomData: 100KB
```

### Scriptless: Request Flow for 1000 Nodes

```
User: Scale node pool to 1000 nodes (K8s 1.29.0)
  │
  ├─> AKS-RP receives request
  │
  ├─> One-time: Query VHD image version (optional, can be cached)
  │   │
  │   ├─> HTTP GET to agentbaker-sig-image-svc
  │   │   URL: /getlatestsigimageconfig
  │   │   Response: {"ubuntu2204": {"2025.02.10": {...}}}
  │   │
  │   └─> Cache result (reuse for all 1000 nodes)
  │
  ├─> For each of 1000 nodes:
  │   │
  │   ├─> Generate AKSNodeConfig CLIENT-SIDE (no service call!)
  │   │   {
  │   │     "version": "v1",
  │   │     "kubernetesVersion": "1.29.0",
  │   │     "localDnsProfile": {"enableHostsPlugin": true}
  │   │   }
  │   │
  │   ├─> Generate CustomData CLIENT-SIDE (~5KB)
  │   │   ├─> Base64 encode JSON config
  │   │   └─> Wrap in cloud-init YAML
  │   │
  │   ├─> Generate CSE CLIENT-SIDE (constant string)
  │   │   "/opt/azure/containers/aks-node-controller provision-wait"
  │   │
  │   └─> Send directly to ARM for VM creation
  │
  └─> Result:
      - 1 HTTP call to AgentBaker (or 0 if cached)
      - ~5MB traffic (1000 × 5KB)
      - AgentBaker service minimally loaded
      - Each CustomData: 5KB
```

**Traffic Comparison:**
- Non-scriptless: **1000 calls × 100KB = 100MB**
- Scriptless: **1 call × 5KB = 5KB** (99.995% reduction!)

---

## Code Generation Comparison

### Non-Scriptless: How Variables are Generated

**File:** `pkg/agent/baker.go:1225-1240`

```go
// GetNodeBootstrappingCmd generates CSE command with templated variables
func (baker *AgentBaker) GetNodeBootstrappingCmd(...) {
    // Generate hosts plugin variables via Go template functions
    vars["SHOULD_ENABLE_HOSTS_PLUGIN"] = baker.profile.ShouldEnableHostsPlugin()
    vars["LOCALDNS_GENERATED_COREFILE"] = baker.generateLocalDNSCoreFile(true)
    vars["LOCALDNS_GENERATED_COREFILE_NO_HOSTS"] = baker.generateLocalDNSCoreFile(false)

    // Template replacement in cse_cmd.sh
    // {{ShouldEnableHostsPlugin}} → "true"
    // {{GetGeneratedLocalDNSCoreFile}} → "<base64>"

    // Generate large CustomData with all scripts embedded
    customData := baker.templateCustomData(vars)

    return CustomData, CSE
}
```

**Result:** Large monolithic CustomData with everything embedded

---

### Scriptless: How Variables are Generated

**File:** `aks-node-controller/parser/parser.go:175-179`

```go
// GetCSEVariables converts AKSNodeConfig to environment variables
func GetCSEVariables(config *aksnodeconfigv1.Configuration) map[string]string {
    envVars := make(map[string]string)

    // Convert Protobuf fields to env vars
    envVars["SHOULD_ENABLE_HOSTS_PLUGIN"] = shouldEnableHostsPlugin(config)
    envVars["LOCALDNS_GENERATED_COREFILE"] = getLocalDnsCorefileBase64WithHostsPlugin(config, true)
    envVars["LOCALDNS_GENERATED_COREFILE_NO_HOSTS"] = getLocalDnsCorefileBase64WithHostsPlugin(config, false)

    return envVars
}
```

**File:** `aks-node-controller/parser/helper.go:809-811`

```go
func shouldEnableHostsPlugin(config *aksnodeconfigv1.Configuration) string {
    return fmt.Sprintf("%v",
        shouldEnableLocalDns(config) == "true" &&
        config.GetLocalDnsProfile().GetEnableHostsPlugin())
}
```

**Runtime (on VM):**
```bash
# aks-node-controller reads config JSON
# Calls parser.GetCSEVariables()
# Exports environment variables
export SHOULD_ENABLE_HOSTS_PLUGIN="true"
export LOCALDNS_GENERATED_COREFILE="<base64>"
export LOCALDNS_GENERATED_COREFILE_NO_HOSTS="<base64>"

# Then executes bash scripts (same scripts as non-scriptless!)
source /opt/azure/containers/cse_main.sh
```

**Result:** Small CustomData, scripts already on disk

---

## Why Both Paths Coexist Right Now

### The 6-Month VHD Support Window

```
Timeline of VHD Lifecycle:
═══════════════════════════════════════════════════════════════════════

Month 0: New VHD v2025.02.10 released
├─> Contains: aks-node-controller binary + all scripts
├─> Supports: Scriptless provisioning
└─> Also supports: Non-scriptless (for compatibility)

Month 1-6: Both VHD versions in production
├─> Old VHD v2024.09.15 (no aks-node-controller)
│   └─> Can ONLY use non-scriptless
│
├─> New VHD v2025.02.10 (has aks-node-controller)
│   ├─> Can use scriptless (preferred)
│   └─> Can use non-scriptless (fallback)
│
└─> Control plane must support BOTH paths

Month 6+: Old VHD aged out of production
├─> All VHDs have aks-node-controller
├─> Can deprecate non-scriptless path
└─> Remove 26+ service pods
```

### Current State: Dual Implementation Required

**For ANY new feature (like EnableHostsPlugin):**

```
Implementation Checklist:
├─ 🔴 Non-Scriptless Path:
│   ├─> Add field to datamodel.LocalDNSProfile
│   ├─> Add template function in baker.go
│   ├─> Generate variables in GetNodeBootstrappingCmd()
│   └─> Test: Test_Ubuntu2204, Test_ACL, Test_Flatcar
│
└─ 🟢 Scriptless Path:
    ├─> Add field to proto LocalDnsProfile
    ├─> Run: make gen (regenerate protobuf)
    ├─> Add parser helper in parser/helper.go
    ├─> Add to env var map in parser/parser.go
    └─> Test: Test_Ubuntu2204_Scriptless, Test_ACL_Scriptless

Both paths produce IDENTICAL runtime behavior:
  └─> Same environment variables → Same bash scripts → Same result
```

---

## Migration Strategy

### Phase 1: Current (Dual Mode)
```
Code Base:
├─ pkg/agent/baker.go (non-scriptless generation)
├─ aks-node-controller/parser/ (scriptless generation)
├─ parts/linux/cloud-init/artifacts/*.sh (shared bash scripts)
└─ Both paths maintained in parallel

Production:
├─ Old VHDs: Use non-scriptless (26+ pods handling requests)
├─ New VHDs: Use scriptless (1 pod or client-side)
└─ AgentBaker pods still deployed (for old VHD support)
```

### Phase 2: Scriptless Adoption (In Progress)
```
Code Base:
├─ aks-node-controller becomes primary
├─ Non-scriptless marked deprecated
└─ Both still tested

Production:
├─ Majority use scriptless
├─ Small % still on old VHDs (non-scriptless)
└─ Can start scaling down AgentBaker pods
```

### Phase 3: Complete Migration (Future)
```
Code Base:
├─ Remove pkg/agent/baker.go template generation
├─ Remove apiserver/getnodebootstrapdata.go
├─ Keep only aks-node-controller path
└─ Potentially replace bash scripts with Go code

Production:
├─ All VHDs have aks-node-controller
├─ Decommission 26+ AgentBaker service pods
└─> 💰 Cost savings + 📉 Reduced complexity
```

---

## Visual Comparison: What Gets Deployed Where

### Non-Scriptless: Scripts Uploaded Per Node

```
┌──────────────────────────────────────────────────────────────┐
│  VHD Image (Base Image)                                      │
│                                                              │
│  Contains:                                                   │
│  ├─> OS (Ubuntu 22.04)                                       │
│  ├─> Kubelet binary (pre-installed)                          │
│  ├─> Containerd binary (pre-installed)                       │
│  ├─> Some helper scripts (minimal)                           │
│  └─> NO provisioning scripts (uploaded per node)             │
│                                                              │
└──────────────────────────────────────────────────────────────┘
                           │
                           │ VM Created from VHD
                           │
┌──────────────────────────▼───────────────────────────────────┐
│  Running VM (Node)                                           │
│                                                              │
│  CustomData (uploaded at provision time, ~100KB):           │
│  ├─> provision.sh                                            │
│  ├─> provision_source.sh                                     │
│  ├─> cse_main.sh                                             │
│  ├─> cse_config.sh                                           │
│  ├─> cse_helpers.sh                                          │
│  ├─> cse_cmd.sh (with templated variables)                  │
│  ├─> aks-hosts-setup.sh                                      │
│  └─> All other CSE scripts                                   │
│                                                              │
│  Scripts written to: /opt/azure/containers/                  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Every node gets fresh scripts uploaded via CustomData**

---

### Scriptless: Scripts Pre-Installed on VHD

```
┌──────────────────────────────────────────────────────────────┐
│  VHD Image (Base Image) - Built with aks-node-controller    │
│                                                              │
│  Contains:                                                   │
│  ├─> OS (Ubuntu 22.04)                                       │
│  ├─> Kubelet binary                                          │
│  ├─> Containerd binary                                       │
│  ├─> aks-node-controller binary (Go binary)                 │
│  ├─> ALL provisioning scripts (pre-installed):              │
│  │   ├─> /opt/azure/containers/cse_main.sh                  │
│  │   ├─> /opt/azure/containers/cse_config.sh                │
│  │   ├─> /opt/azure/containers/cse_helpers.sh               │
│  │   ├─> /opt/azure/containers/aks-hosts-setup.sh           │
│  │   └─> All other CSE scripts                              │
│  └─> Systemd units:                                          │
│      ├─> aks-node-controller.service                         │
│      └─> aks-hosts-setup.timer                               │
│                                                              │
└──────────────────────────────────────────────────────────────┘
                           │
                           │ VM Created from VHD
                           │
┌──────────────────────────▼───────────────────────────────────┐
│  Running VM (Node)                                           │
│                                                              │
│  CustomData (uploaded at provision time, ~5KB):             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ #cloud-config                                          │ │
│  │ write_files:                                           │ │
│  │ - path: /opt/azure/containers/                         │ │
│  │         aks-node-controller-config.json                │ │
│  │   content: !!binary |                                  │ │
│  │     ewogICJ2ZXJzaW9uIjogInYxIiwKICAia3ViZXJuZXRl... │ │
│  │     (Just base64 encoded config JSON)                  │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Scripts already on disk (from VHD):                         │
│  ✅ /opt/azure/containers/aks-node-controller               │
│  ✅ /opt/azure/containers/cse_main.sh                       │
│  ✅ /opt/azure/containers/aks-hosts-setup.sh                │
│  ✅ ... (all scripts pre-installed)                          │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Scripts come from VHD, only config JSON uploaded**

---

## Cost and Efficiency Comparison

### Resource Usage: 1000 Node Scale-Up

| Metric | Non-Scriptless | Scriptless | Improvement |
|--------|---------------|------------|-------------|
| **AgentBaker API Calls** | 1,000 calls | 0-1 calls | 99.9% reduction |
| **Network Traffic** | ~100MB | ~5KB-5MB | 95-99.995% reduction |
| **CustomData per Node** | ~100KB | ~5KB | 95% reduction |
| **AgentBaker Pod Load** | 1,000 requests to process | 0-1 requests | 99.9% reduction |
| **CSE Execution Time** | Similar | Similar | No change (same scripts) |
| **Total Pods Required** | 78-130 pods (26 deployments × 3-5 replicas) | 3-5 pods (1 deployment) | 95% reduction |

### Operational Cost Comparison

**Non-Scriptless:**
```
Kubernetes Cluster Resources:
├─> 26 Deployments
├─> 78-130 Pods (@ 3-5 replicas each)
├─> CPU: ~5-10 cores total (26 × 0.2-0.4 cores)
├─> Memory: ~10-20GB total (26 × 400-800MB)
├─> Load Balancer: Complex routing by K8s version
├─> Monitoring: 26 different services to monitor
└─> Updates: Deploy 26 services for each AgentBaker release

Operations Cost:
├─> High network egress (large CustomData payloads)
├─> High compute (processing 1000s of requests)
└─> High maintenance (26+ services to manage)
```

**Scriptless:**
```
Kubernetes Cluster Resources:
├─> 1 Deployment (or none - can be fully client-side)
├─> 3-5 Pods (1 deployment for VHD metadata)
├─> CPU: ~0.2-0.5 cores total
├─> Memory: ~400-800MB total
├─> Load Balancer: Simple (single service)
├─> Monitoring: 1 service to monitor
└─> Updates: Deploy 1 service for AgentBaker releases

Operations Cost:
├─> Low network egress (tiny config payloads)
├─> Low compute (minimal requests)
└─> Low maintenance (1 service or none)

💰 Estimated savings: 90-95% reduction in infrastructure cost
```

---

## Why the Migration Takes Time

### Challenge 1: VHD Lifecycle

```
┌────────────────────────────────────────────────────────────────┐
│  VHD Support Window: 6 Months                                  │
│                                                                │
│  Jan 2025: Release VHD v2025.01.15                             │
│    ├─> Contains: aks-node-controller                           │
│    └─> Supports: Scriptless                                    │
│                                                                │
│  Feb 2025: Some production clusters still on v2024.08.10       │
│    ├─> Lacks: aks-node-controller                              │
│    └─> Requires: Non-scriptless                                │
│                                                                │
│  Jul 2025: v2024.08.10 reaches end of support                  │
│    └─> Can deprecate non-scriptless (all VHDs have controller) │
│                                                                │
│  Must maintain both paths for 6 months after first release!   │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### Challenge 2: Testing Matrix

```
Test Matrix (Must Pass All):
├─ Non-Scriptless:
│   ├─> Test_Ubuntu2204
│   ├─> Test_Ubuntu2404
│   ├─> Test_AzureLinux
│   ├─> Test_Flatcar
│   └─> ... (all OS variants)
│
└─ Scriptless:
    ├─> Test_Ubuntu2204_Scriptless
    ├─> Test_Ubuntu2404_Scriptless
    ├─> Test_AzureLinux_Scriptless
    ├─> Test_Flatcar_Scriptless
    └─> ... (duplicate tests for all OS variants)

Result: 2x test coverage during migration
```

### Challenge 3: Feature Parity

```
Every feature must be implemented twice:
├─ datamodel.NodeBootstrappingConfiguration (non-scriptless)
└─ aksnodeconfigv1.Configuration (scriptless)

Example: EnableHostsPlugin
├─ Non-scriptless:
│   ├─> datamodel.LocalDNSProfile.EnableHostsPlugin
│   ├─> baker.go template function
│   └─> Template: {{ShouldEnableHostsPlugin}}
│
└─ Scriptless:
    ├─> proto LocalDnsProfile.enable_hosts_plugin
    ├─> parser helper: shouldEnableHostsPlugin()
    └─> Runtime: export SHOULD_ENABLE_HOSTS_PLUGIN

Both produce: SHOULD_ENABLE_HOSTS_PLUGIN="true" at runtime
```

---

## Convergence Point: Bash Scripts

**Critical Insight:** Both paths execute **identical bash scripts** at runtime!

```
┌─────────────────────────────────────────────────────────────────┐
│  Configuration Generation (DIVERGES)                            │
│                                                                 │
│  Non-Scriptless:          Scriptless:                           │
│  pkg/agent/baker.go       aks-node-controller/parser/           │
│         │                          │                            │
│         ├─> Go templates           ├─> Protobuf + parser        │
│         │                          │                            │
│         ▼                          ▼                            │
│  Generate env vars:        Generate env vars:                   │
│  SHOULD_ENABLE_            SHOULD_ENABLE_                       │
│  HOSTS_PLUGIN="true"       HOSTS_PLUGIN="true"                  │
│                                                                 │
└─────────────┬───────────────────────┬───────────────────────────┘
              │                       │
              └───────────┬───────────┘
                          │
          ┌───────────────▼───────────────┐
          │  CONVERGENCE POINT            │
          │  (IDENTICAL EXECUTION)        │
          └───────────────┬───────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│  Bash Script Execution (IDENTICAL)                              │
│  parts/linux/cloud-init/artifacts/                              │
│                                                                 │
│  cse_main.sh:                                                   │
│  ├─> enableAKSHostsSetup()                                      │
│  │   ├─> Validate artifacts                                     │
│  │   ├─> Write /etc/localdns/cloud-env                          │
│  │   ├─> Create /etc/localdns/hosts                             │
│  │   └─> Enable aks-hosts-setup.timer                           │
│  │                                                              │
│  └─> select_localdns_corefile()                                 │
│      ├─> Wait for hosts file                                    │
│      └─> Choose WITH_HOSTS or NO_HOSTS Corefile                 │
│                                                                 │
│  Result: Node bootstrapped and joined to cluster                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Point:** The diversion is **ONLY** in configuration generation. Runtime execution is **IDENTICAL**.

---

## Summary: Why the Diversion Exists

### The Simple Answer

**AgentBaker is migrating from a 26-pod architecture to a 1-pod (or 0-pod) architecture.**

- **Non-scriptless** = Current production system (expensive, complex)
- **Scriptless** = Future architecture (efficient, simple)
- **Both paths** = Required during 6-month transition period

### The Diversion is Temporary

```
2024: Non-scriptless only (26+ pods)
  │
2025: Dual mode (both paths, 26+ pods still running)
  │   ├─> All new features implemented twice
  │   ├─> Both test suites maintained
  │   └─> Code complexity doubled temporarily
  │
2026: Scriptless majority (scaling down pods)
  │   ├─> Most traffic uses scriptless
  │   └─> Non-scriptless marked deprecated
  │
2027: Scriptless only (1 pod or client-side)
      ├─> Remove non-scriptless code
      ├─> Decommission 26+ service pods
      └─> 💰 Massive cost savings
```

**For your specific question about EnableHostsPlugin:** You had to implement it in both paths because we're in the middle of this migration. Once scriptless is fully adopted, you'd only implement it once (in the proto + parser).

---

## Quick Reference: Where Things Live

### Non-Scriptless Code
- **API Service:** `apiserver/getnodebootstrapdata.go`
- **Config Struct:** `pkg/agent/datamodel/types.go`
- **Code Generation:** `pkg/agent/baker.go` (template functions)
- **Running In:** 26+ pods in Azure backend (agentbaker-v1-xx-xx)

### Scriptless Code
- **API Service:** `apiserver/getlatestsigimageconfig.go` (metadata only)
- **Config Contract:** `aks-node-controller/proto/aksnodeconfig/v1/*.proto`
- **Code Generation:** `aks-node-controller/parser/parser.go` + `helper.go`
- **Runtime Binary:** `/opt/azure/containers/aks-node-controller` (on VM)
- **Running In:** 1 pod in Azure backend (or client-side, no pod needed)

### Shared Code (Both Use)
- **Bash Scripts:** `parts/linux/cloud-init/artifacts/*.sh`
- **Execution:** Identical at runtime (both paths run same scripts)
- **VHD:** Scripts baked into VHD during build

### The Key Difference
- **Non-scriptless:** Scripts come FROM AgentBaker service (via CustomData)
- **Scriptless:** Scripts come FROM VHD (pre-installed), config comes via CustomData

---

Does this clarify where the 26 pods are and why we have both paths?
