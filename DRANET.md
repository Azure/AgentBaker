Managed DRANET in AKS 

Overview 

DRANET is the AKS-managed Network Dynamic Resource Allocation (DRA) driver responsible for discovering, advertising, and managing RDMA-capable network devices on eligible AKS nodes. It runs as a privileged Kubernetes DaemonSet on RDMA-capable Linux node pools where NvidiaRDMAProfile.managementMode is set to Managed.  

 

At a high level, DRANET performs the following functions on each eligible node: 

 

1. Device Discovery and Validation 
 

On startup, the DRANET daemon inspects the node for RDMA-capable NVIDIA networking devices (e.g., InfiniBand NICs exposed via Mellanox OFED). It validates: 

Presence of RDMA devices (e.g., /dev/infiniband) 

Required kernel modules and driver state 

Compatibility with the AKS-managed Mellanox OFED driver stack 

Nodes that fail validation do not advertise RDMA resources, preventing partial or unsafe allocations.  

 

2. Resource Advertisement via Kubernetes DRA 

 
Once devices are discovered, DRANET advertises them to Kubernetes using the DRA API: 

A DeviceClass / ResourceClass defines the type of RDMA-capable network resource. 

ResourceSlices represent the concrete RDMA devices available on a specific node. These objects allow the Kubernetes scheduler to make placement decisions that are aware of both compute and RDMA networking constraints.  

 

3. Allocation and Lifecycle Management 

 
When a pod requests RDMA resources via a ResourceClaimTemplate, the scheduler binds the claim to an available RDMA device on a node. DRANET then: 

Attaches the allocated device to the pod 

Ensures exclusive access where required (one NIC per pod) 

Cleans up and re-advertises resources when the pod terminates 

This lifecycle integrated with standard Kubernetes pod scheduling and deletion semantics, avoiding stale or leaked RDMA allocations during scale, upgrade, or failure scenarios.  

 

4. Integration with AKS Networking and CNI 
DRANET is not responsible for the primary pod network. Cilium (or the configured AKS CNI) continues to manage the default NIC and pod-to-pod networking. DRANET exclusively manages additional RDMA-capable NICs used for high-performance data paths (e.g., NCCL, MPI, distributed inference).  

 

 

DRANET is deployed via Eno Synth as a Kubernetes DaemonSet to RDMA-capable AKS node pools. It integrates with AKS-managed Mellanox OFED drivers and Kubernetes Dynamic Resource Allocation (DRA) to expose RDMA-capable NICs as schedulable resources. 

Deployment Model 

Managed RDMA enablement 

The ManagedCluster property NvidiaRDMAProfile.managementMode will control Managed DRANET enablement. Opt-in in 1.35, defaults to “Managed” in 1.36.  

 

See API Sample. 

 

OFED Drivers 

Decision needed. 

Two options for installing OFED drivers are available: 

Drivers are baked in the VHD (AgentBaker). 

Drivers are installed by the DRANET daemonset via privileged init-container (DRANET Synth) 

 

Via AgentBaker (preferred) 

Pros 

Node is ready to go when it boots. 

No runtime driver install. 

Less-privileged DaemonSet. 

Cons 

Driver updates require reimaging. 

Driver updates/DRANET requirements may drift out of sync. 

Via DRANET Synth 

Pros 

Driver can be updated on demand. 

Cons 

Larger, more-privileged DaemonSet. 

Runtime driver installation may be disruptive. 

Delay in Node readiness due to driver installation. 

Need a solution to avoid repeatedly reinstalling drivers during DRANET DaemonSet rollouts. 

DRANET Synth 

When Managed RDMA is enabled, it will enable the DRANET Synth which will install the Daemonset. 

 

The DRANET Synth will install DRANET in alignment with the public upstream deployment, using Microsoft’s build of the DRANET artifacts to bring any private patches or fixes that have not yet merged upstream. 

 

See Deployment Sample. 

 

DRANET Daemonset 

If OFED driver installation is via DRANET Synth, will contain a custom init which installs that driver when necessary. 

 

The daemonset will selectively schedule on RDMA-capable Nodes. 

Node label applied to APs of RDMA-capable SKUs by AKS  

Node-affinity targeting RDMA label in the DRANET daemonset 

 

Suggested Node label: kubernetes.azure.com/network-dra=rdma 

 

Requirements: AKS keeps the RDMA-capable SKU mapping and applies an RDMA label to nodepools of those SKUs. 

 

Design Discussions 

Default vs Opt-Out Behavior 

Kubernetes v1.35: RDMA management is Unmanaged by default (explicit opt-in required). 

Kubernetes v1.36+: RDMA management is Managed by default, with opt-out available at the node pool level. 

 

Deployment, Upgrade, and Failure Scenarios 

DRANET follows standard Kubernetes DaemonSet rollout semantics. Mixed-mode nodes (some managed, some unmanaged) are not supported within the same node pool. During upgrades or scale operations, RDMA resources re-advertised to avoid stale allocations. 

 

Security and Compliance 

DRANET runs with elevated privileges required for device management. Secure Boot, vTPM, FIPS images, and CVM scenarios are not supported due to unsigned NVIDIA networking drivers. CVE and security fixes to DRANET are delivered via DaemonSet updates instead of node image upgrades (and OFED driver depending on delivery design). 

 

Observability and Validation 

AKS E2E pipelines and network hypercube validate device presence, kernel module state, DRANET health, and correct RDMA resource exposure.  

DRANET health is monitored using a combination of Kubernetes-native signals, AKS platform validation, and workload-level verification. 

 

DaemonSet and Pod Health 

DRANET exposes a /healthz HTTP endpoint used by Kubernetes readiness probes. 

A node is a candidate for RDMA scheduling only once the DRANET pod has published the device resources. 

Standard DaemonSet status fields (desired, ready, available) provide rollout and upgrade health visibility.  

Resource Advertisement Validation AKS E2E pipelines validate: 

RDMA devices are present and enumerated 

OFED drivers are loaded 

RDMA resources are published as DRA ResourceSlices 

If resources are missing or withdrawn, the Kubernetes scheduler will naturally stop placing RDMA workloads on affected nodes. 

Upgrade and Failure Handling During node upgrades, scale events, or DaemonSet updates: 

DRANET deployment follows standard Kubernetes rolling update semantics 

DRANET unavailability (crash, rolling upgrade) still allows Pods to schedule but they will stay Pending until the DRANET socket is recreated 

Releases roll via SDP/standard regional health signals 

Telemetry collected by standard AKS tools 

Prometheus metrics via aks-operator 

Logs: 

AKS solution for collecting logs from cx Nodes yet? 

Geneva action similar to CNS  

Otherwise, ACN appinsights sidecar like in use for Cilium 

Reference Samples and Architecture 

API Sample 

"ManagedClusterAgentPoolProfileProperties": { 

… 

+   "NvidiaRDMAProfile": { 

+       "type": "object", 

+       "description": "NVIDIA-specific RDMA settings.", 

+       "properties": { 

+         "managementMode": { 

+           "type": "string", 

+            "enum": [ 

+             "Unmanaged", 

+             "Managed" 

+           ], 

+           "x-ms-enum": { 

+             "name": "ManagementMode", 

+             "modelAsString": true, 

+             "values": [ 

+               { 

+                 "value": "Unmanaged", 

+                 "description": "Managed RDMA experience is disabled for NVIDIA RDMA skus." 

+               }, 

+               { 

+                 "value": "Managed", 

+                 "description": "Managed RDMA experience is enabled for NVIDIA RDMA skus." 

+               } 

+             ] 

+           }, 

+           "description": "The Managed RDMA experience installs additional components, such as the DRANET driver for Kubernetes configuration, on top of the Mellanox kernel driver.", 

+           "default": "Managed"     long term goal is managed by default 

+          } 

+        }, 

… 

… 

 

 

 

Deployment Sample 

# Copyright The Kubernetes Authors 

# 

# Licensed under the Apache License, Version 2.0 (the "License"); 

# you may not use this file except in compliance with the License. 

# You may obtain a copy of the License at 

# 

#    https://www.apache.org/licenses/LICENSE-2.0 

# 

# Unless required by applicable law or agreed to in writing, software 

# distributed under the License is distributed on an "AS IS" BASIS, 

# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. 

# See the License for the specific language governing permissions and 

# limitations under the License. 

--- 

kind: ClusterRole 

apiVersion: rbac.authorization.k8s.io/v1 

metadata: 

  name: dranet 

rules: 

  - apiGroups: 

      - "" 

    resources: 

      - nodes 

    verbs: 

      - get 

  - apiGroups: 

      - "resource.k8s.io" 

    resources: 

      - resourceslices 

    verbs: 

      - list 

      - watch 

      - create 

      - update 

      - delete 

  - apiGroups: 

      - "resource.k8s.io" 

    resources: 

      - resourceclaims 

      - deviceclasses 

    verbs: 

      - get 

  - apiGroups: 

      - "resource.k8s.io" 

    resources: 

      - resourceclaims/status 

    verbs: 

      - patch 

      - update 

--- 

 

kind: ClusterRoleBinding 

apiVersion: rbac.authorization.k8s.io/v1 

metadata: 

  name: dranet 

roleRef: 

  apiGroup: rbac.authorization.k8s.io 

  kind: ClusterRole 

  name: dranet 

subjects: 

- kind: ServiceAccount 

  name: dranet 

  namespace: kube-system 

--- 

apiVersion: v1 

kind: ServiceAccount 

metadata: 

  name: dranet 

  namespace: kube-system 

--- 

apiVersion: apps/v1 

kind: DaemonSet 

metadata: 

  name: dranet 

  namespace: kube-system 

  labels: 

    tier: node 

    app: dranet 

    k8s-app: dranet 

spec: 

  selector: 

    matchLabels: 

      app: dranet 

  template: 

    metadata: 

      labels: 

        tier: node 

        app: dranet 

        k8s-app: dranet 

    spec: 

      hostNetwork: true 

      tolerations: 

      - operator: Exists 

        effect: NoSchedule 

      affinity: 

        nodeAffinity: 

          requiredDuringSchedulingIgnoredDuringExecution: 

            nodeSelectorTerms: 

            - matchExpressions: 

              - key: kubernetes.azure.com/network-dra 

                operator: In 

                values: 

                - rdma 

      serviceAccountName: dranet 

      hostPID: true 

      initContainers: 

      - name: enable-nri 

        image: busybox:stable 

        volumeMounts: 

        - mountPath: /etc 

          name: etc 

        securityContext: 

          privileged: true 

        command: 

        - /bin/sh 

        - -c 

        - | 

          set -o errexit 

          set -o pipefail 

          set -o nounset 

          set -x 

          if grep -q "io.containerd.nri.v1.nri" /etc/containerd/config.toml 

          then 

             echo "containerd config contains NRI reference already; taking no action" 

          else 

             echo "containerd config does not mention NRI, thus enabling it"; 

             printf '%s\n' "[plugins.\"io.containerd.nri.v1.nri\"]" "  disable = false" "  disable_connections = false" "  plugin_config_path = \"/etc/nri/conf.d\"" "  plugin_path = \"/opt/nri/plugins\"" "  plugin_registration_timeout = \"5s\"" "  plugin_request_timeout = \"5s\"" "  socket_path = \"/var/run/nri/nri.sock\"" >> /etc/containerd/config.toml 

             echo "restarting containerd" 

             nsenter -t 1 -m -u -i -n -p -- systemctl restart containerd 

          fi 

      - name: install-ofed-driver 

        image: <TODO: ofed-installer-image> 

        securityContext: 

          privileged: true 

        command: 

        - /bin/sh 

        - -c 

        - | 

          # TODO: If OFED drivers are installed by this DaemonSet, implement idempotent install logic here. 

          # TODO: Ensure no-op semantics on restart/rollout (do not repeatedly reinstall). 

          echo \"OFED driver install placeholder (no-op)\" 

          exit 0 

      containers: 

      - name: dranet 

        args: 

        - /dranet 

        - --v=4 

        - --hostname-override=$(NODE_NAME) 

        image: acnpublic.azurecr.io/dranet:dev8 

        env: 

        - name: NODE_NAME 

          valueFrom: 

            fieldRef: 

              fieldPath: spec.nodeName 

        resources: 

          requests: 

            cpu: "100m" 

            memory: "50Mi" 

        securityContext: 

          privileged: true 

        readinessProbe: 

          httpGet: 

            path: /healthz 

            port: 9177 

        volumeMounts: 

        - name: device-plugin 

          mountPath: /var/lib/kubelet/plugins 

        - name: plugin-registry 

          mountPath: /var/lib/kubelet/plugins_registry 

        - name: nri-plugin 

          mountPath: /var/run/nri 

        - name: netns 

          mountPath: /var/run/netns 

          mountPropagation: HostToContainer 

        - name: infiniband 

          mountPath: /dev/infiniband 

          mountPropagation: HostToContainer 

        - name: bpf-programs 

          mountPath: /sys/fs/bpf 

          mountPropagation: HostToContainer 

      volumes: 

      - name: device-plugin 

        hostPath: 

          path: /var/lib/kubelet/plugins 

      - name: plugin-registry 

        hostPath: 

          path: /var/lib/kubelet/plugins_registry 

      - name: nri-plugin 

        hostPath: 

          path: /var/run/nri 

      - name: netns 

        hostPath: 

          path: /var/run/netns 

      - name: infiniband 

        hostPath: 

          path: /dev/infiniband 

      - name: etc 

        hostPath: 

          path: /etc 

      - name: bpf-programs 

        hostPath: 

          path: /sys/fs/bpf 

Architecture 

 

 

 

 

Cilium continues to handle provisioning of the default network interface card (NIC). DRANET, running as a daemonset, is responsible for adding extra devices or NICs inside pods. When DRANET operates only on RDMA-enabled SKUs, it identifies every device present on the node and lists them as resource slice entries. Users can then generate resource claims or templates and reference them in their pod specifications to allocate the RDMA devices needed for a particular pod or deployment. 

After a user creates a pod, the scheduler selects a node that meets the specified requirements. The kubelet on that node initiates a prepareresourceclaim API call, which performs most of the setup in the host namespace by collecting configuration information to be applied when the pod sandbox launches (via NRI hooks). During container creation, the container-create NRI hook assigns the relevant RDMA character devices to the container. Users may request multiple RDMA NICs per pod, and the DRANET driver assigns an equal number of character devices to each pod. 

 

 

 

Workstreams 

 

Open Decisions: 

OFED Driver Delivery 

RDMA Label 

Log/Metric collector 

 

MVP: 

Artifact prep: ✅ 

Private fork of upstream dranet 

MS compliant image build/release  

Published to MCR  

Runtime:  

Managed DRANET Synth  

Create ✅ 

Register 

Test   

Create release defs/pipelines 

RDMA Node labelling in AKS-RP 

OFED driver installation in AB or init 

Profiling to set resource constraints 

Observability:  

Create AppInsights resource 

Prepare Logs sidecar 

Works as is? Refactor  

Prometheus metrics collection via aks-operator 

Alertmonitor integration and alerting setup via aks-operator 

Validation 

UTs for synth 

E2Es in PR 

Added to hypercube matrix 

Healthchecks for release rollout 

 

 

 

Product decisions & API/UX 

Finalize NvidiaRDMAProfile.managementMode semantics (1.35 opt-in vs 1.36 default), node-pool opt-out behavior, supported SKUs/OS/K8s matrix, and customer docs for enablement + troubleshooting. 

OFED driver delivery 

Decision and implementation for AgentBaker-baked vs DaemonSet-installed OFED; upgrade/rollback plan; compatibility checks; CVE process; “driver drift” detection and remediation. 

Node eligibility & scheduling 

RDMA SKU mapping and reliable node labeling; DaemonSet node affinity/taints strategy; mixed-pool behavior during scale/upgrade/repair; optional additional constraints (OS/arch/kernel). 

DRANET DaemonSet hardening 

Least-privilege review (host mounts/hostPID/privileged); RBAC minimization; readiness/liveness criteria tied to ResourceSlice publication and NRI functionality; operational runbook. 

Rollout, upgrades & failure modes 

Define behavior for DaemonSet rollouts, node upgrades, crashes, and partial failures; ensure no stale allocations; safe rollout gates (rings/SDP), rollback criteria, and incident playbooks. 

Observability & supportability 

Metrics (discovery counts, ResourceSlice publish success, claim prep latency/errors); logs collection path for RDMA nodes; actionable events/conditions (driver missing/mismatch); dashboards and alerts. 

Validation & E2E testing 

E2E suites for discovery/advertisement/allocation/cleanup, disruptive scenarios (upgrade/scale/rollout), and negative tests (mislabeled nodes, missing drivers, permissions). Workload-level validation (NCCL/MPI/distributed inference patterns). 

Artifact supply chain & release 

Signed/scanned images; versioning and compatibility policy (K8s/DRANET/OFED/node image); promotion process from dev to prod. 

Compliance & constrained env support 

Document and enforce unsupported scenarios (Secure Boot, vTPM, FIPS, CVM); add preflight checks and user-facing errors; security review sign-offs for privileged components. 

Suggested milestone sequence 

M0 – Decisions: finalize managementMode semantics and the OFED delivery approach (AgentBaker vs DS install), plus the supported matrix. 

M1 – Minimum viable managed rollout: node labeling + DaemonSet scheduling, device discovery/validation, ResourceSlice publication, and a basic end-to-end RDMA workload validation. 

M2 – Upgrade & reliability: cover node image upgrades, DaemonSet upgrades, scale in/out, and crash/restart scenarios; prove no leaked/stale allocations. 

M3 – Security & compliance readiness: complete privilege/RBAC hardening, document/enforce constraints (Secure Boot/vTPM/FIPS/CVM), and complete required security reviews. 

M4 – Observability & support: production logging/metrics, dashboards/alerts, and an operational runbook with clear troubleshooting signals. 

M5 – Release readiness: supply-chain gates (signing/scanning), ringed rollout plan (SDP), rollback criteria, and customer-facing documentation. 

 