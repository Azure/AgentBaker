# AKS RCV v1 blast radius

## Purpose

AKS RCV v1 imports first-party CA certificates published through WireServer into
the node operating system trust store. Updating the files on disk does not
guarantee that an already-running process will use the new trust set:

- a process may use a trust bundle baked into its container image;
- a process may use an explicit CA file instead of the system trust store; or
- a long-running process may retain an in-memory certificate pool until it is
  restarted.

This document defines the v1 blast radius and divides ownership between
node-level processes, AKS-owned pods, and customer-owned pods. The initial
implementation is Linux-focused. Windows trust propagation and process reload
behavior require a separate inventory and validation.

The scope applies to both CA additions and removals. Additions primarily create
availability risk when a process cannot trust a newly issued certificate.
Removals also create security risk because a process may continue trusting a
root that has been removed from the on-disk trust store.

## Scope summary

| Scope | V1 responsibility | Primary remediation |
|---|---|---|
| Node-level processes | AgentBaker owns trust installation, containerd recovery, and the host-process audit | Use the updated native system trust store, then restart or reload only affected host services; controlled node reboot is the fallback |
| AKS-owned pods | AKS add-on owners must identify first-party TLS clients, provide current node trust to them, and reload their processes | Host-mount a normalized node bundle where system trust is intended, then roll the affected managed DaemonSet or Deployment |
| Customer-owned pods | AKS documents the trust contract and supplies a stable opt-in mechanism; customers own application trust and restart behavior | Customer opt-in host mount or private trust configuration, followed by a customer-controlled rollout or application hot reload |

## 1. Node-level scope

### 1.1 Containerd

Containerd is the confirmed v1 failure mode. Linux containerd uses the operating
system trust store for registries without an explicit per-registry CA policy.
Go initializes and caches system roots within the long-running process, so a
later update to the node CA bundle is not necessarily observed by that
containerd invocation.

[AgentBaker PR #9432](https://github.com/Azure/AgentBaker/pull/9432) is the
targeted repair:

- calculate the effective trust-bundle digest during scheduled CA refresh;
- do nothing when trust is unchanged;
- use `systemctl try-restart containerd` when trust changes and containerd is
  active;
- preserve an attempted digest so an interrupted or failed recovery can be
  retried;
- verify that containerd has a new healthy systemd invocation; and
- avoid restarting kubelet, draining the node, rebooting, or modifying an
  owner's explicit registry trust policy.

The checked-in containerd unit uses `KillMode=process`. Existing containers are
expected to survive under their containerd shims while the daemon restarts.
This behavior must remain covered by end-to-end validation across supported
Linux OS and containerd versions.

### 1.2 AgentBaker host-package audit

The audit must be based on runtime behavior, not merely on whether a package is
present in a VHD. A package is in the actionable blast radius only when it:

1. runs as a long-lived process;
2. initiates TLS to a first-party endpoint;
3. uses the node system trust store; and
4. does not reload that trust automatically after an on-disk update.

The initial AgentBaker inventory produces the following groups.

#### Host-process blast-radius matrix

| Component | Process model | First-party dependency | Current trust source | Failure mode after trust change | Detection | Remediation |
|---|---|---|---|---|---|---|
| `containerd` | Long-running host daemon | MCR, ACR, and registry content endpoints | Native node system trust unless an explicit registry policy overrides it | In-memory Go roots remain stale; new image pulls fail | Fresh-process TLS succeeds while an uncached CRI pull fails with unknown authority | **Confirmed v1 action:** conditionally restart containerd through PR #9432 |
| `blobfuse2` | Long-running process per mount | Azure Storage | Expected native node system trust unless mount configuration supplies another CA | Existing mount process may retain stale Go roots | Read-only metadata or file operation on an existing mount, correlated with a fresh-process TLS control | Continue using native system trust; safely recreate/remount the process. Drain the workload or node when the mount cannot be restarted independently |
| `blobfuse` | Long-running process per mount | Azure Storage | Native library/system trust unless configured otherwise | Existing TLS context may retain stale roots | Read-only operation on an existing mount and classified TLS logs | Continue using native system trust; validate library reload support, otherwise safely recreate/remount |
| `aznfs`/`stunnel` | Long-running process per TLS-enabled mount | Azure Storage/NFS endpoints | Native node trust or explicit `stunnel` CA configuration | Existing `stunnel` process may retain stale trust | Existing mount operation plus `stunnel` TLS diagnostics | Reload `stunnel` if supported; otherwise coordinate per-mount restart or node drain |
| `acr-mirror` and OverlayBD | Long-running host services when artifact streaming is enabled | ACR and artifact-streaming storage endpoints | Requires implementation validation | New uncached artifact fetch may fail while cached content continues working | Fetch uncached artifact metadata/content and classify unknown-authority failures | Point the service at native node trust and restart only the artifact-streaming services |
| `walinuxagent` | Long-running host daemon | WireServer and extension/package locations | WireServer control channel is HTTP; HTTPS payload behavior requires validation | HTTPS extension/package download may retain stale trust | Exercise the actual HTTPS download path; do not infer from WireServer HTTP health | Use native node trust and restart the agent only if the HTTPS client is proven to cache roots and restart is operationally safe |
| `kubelet` | Long-running host daemon | Kubernetes API server | Explicit cluster CA at `/etc/kubernetes/certs/ca.crt` | Public first-party CA rotation should not affect the primary API-server path | Existing kubelet readiness and API connection | **No system-trust remediation.** Restart only if a separate system-trust-dependent path is identified |
| `aks-secure-tls-bootstrap-client` | One-shot host service | AKS bootstrap/control-plane endpoint | Explicit `--cluster-ca-file` | No long-lived root pool after the command exits | Command result and generated kubeconfig | **No persistent restart.** Preserve explicit cluster-CA behavior |
| Azure ACR credential provider | On-demand exec plugin | ACR/identity endpoints | Process-local trust loaded for each invocation | Only the active invocation can be stale | Execute a new credential request | **No persistent restart.** New invocation reads current trust |
| `apt`, `dnf`, `tdnf`, and `curl` | Short-lived host commands | Microsoft package and AKS endpoints | Native node system trust | No stale process after command completion | Run a new read-only package metadata or TLS request | **No restart.** New invocation reads current trust |
| `kubectl`, `crictl`, and `oras` | Short-lived host commands | Cluster API, CRI socket, or registry | Explicit cluster CA, local socket, or native system trust depending on command | No persistent stale pool after command completion | Run the relevant command | **No restart.** Keep trust source appropriate to the endpoint |
| CNI binaries and `runc` | Short-lived local commands | Local runtime/network operations | Not normally a first-party public TLS client | No persistent stale pool | Normal pod sandbox/container lifecycle | **No restart.** Audit long-running CNI agents under AKS-owned pods instead |
| `node-exporter` | Long-running host metrics server | Primarily inbound scrape traffic | Server certificate configuration, not outbound first-party trust | No identified first-party client failure | Metrics scrape | **No action** for first-party outbound CA rotation |
| Local DNS, log rotation, mount helpers, DHCP, and local telemetry scripts | Local or short-lived host processes | None identified | Not applicable | No identified first-party TLS failure | Existing component health | **No action** unless a concrete HTTPS dependency is introduced |
| GPU device plugins and local GPU daemons | Long-running local agents | Primarily local GPU APIs | Not established as a first-party TLS client | No identified first-party TLS failure | Existing GPU health checks | **No action** unless a concrete Microsoft HTTPS endpoint is identified |
| Other feature-installed first-party agents | Feature-dependent | Feature-specific Microsoft endpoints | Unknown until audited | Runtime-specific stale trust | Feature-owned read-only operation | Prefer native node trust; define targeted reload/restart before inclusion |

This table is a starting classification, not a substitute for testing.

| Required audit field | Purpose |
|---|---|
| Binary, package, and systemd unit | Establish ownership and exact runtime |
| Process lifetime | Separate persistent clients from commands that naturally reload |
| First-party endpoints | Confirm that the component is in the RCV blast radius |
| Trust source | Distinguish native system, explicit cluster CA, image-baked, and private trust |
| TLS implementation | Determine whether trust is cached and whether reload is supported |
| Safe read-only probe | Verify the actual component path without mutating customer resources |
| Targeted remediation | Prefer reload or component restart over node reboot |
| Addition and removal behavior | Cover both availability and distrust/security requirements |

### 1.3 Detection and node remediation

Node Problem Detector (NPD) can report a component-specific condition, but it
must not reboot the node directly. A fresh `curl` only proves that a newly
started process can read the updated bundle; it does not prove that an existing
daemon refreshed its in-memory roots.

| Containerd detector step | Expected result |
|---|---|
| Fresh-process TLS control using current node trust | Succeeds |
| Uncached CRI registry operation using the running containerd | Fails specifically with unknown certificate authority |
| DNS, timeout, authentication, authorization, expiration, or SAN classification | Excluded from stale-trust remediation |

If targeted remediation cannot recover an affected host component, an external
controller may use the following escalation:

| Step | Action |
|---|---|
| 1 | Rate-limit remediation by node pool and availability zone |
| 2 | Cordon and drain the node while honoring disruption policy |
| 3 | Reboot the node |
| 4 | Validate node readiness and the affected first-party operations |
| 5 | Reimage or redeploy only if reboot does not recover the node |

The CA generation or bundle digest must be recorded to prevent repeated
remediation for the same trust update.

## 2. Pod-level scope

A node trust update does not automatically update pod trust.

| Pod trust source | Update behavior |
|---|---|
| CA bundle baked into the image | Changes only when the image is rebuilt or updated |
| Host-mounted node bundle | File updates become visible, but process-level roots may remain cached |
| Kubernetes Secret or ConfigMap | Follows Kubernetes projection semantics and still may require process reload |
| Explicit application CA file | Changes only through application-owned configuration |
| Java, .NET, Python, Node.js, or other runtime-specific store | Follows runtime-specific update and reload semantics |
| In-memory root pool | Remains stale until explicitly reloaded or the process is recreated |

A hostPath mount makes updated bytes visible inside the container. It does not
force the running process to rebuild an in-memory TLS configuration.

### 2.a AKS-owned pods

AKS owns the complete trust lifecycle for managed add-ons and node agents:

| Responsibility | Required result |
|---|---|
| Inventory | Identify every long-running client that calls a first-party TLS endpoint |
| Trust classification | Record image-baked, host-mounted, explicit, cluster-specific, or private trust |
| Trust delivery | Expose current normalized node trust at the path expected by the container when system trust inheritance is intended |
| Process refresh | Restart or hot-reload after a trust-generation change |
| Validation | Exercise the real component operation rather than only local liveness |

The initial managed-pod inventory is:

| Component | Process and first-party TLS path | Current trust source | Failure mode and detection | Remediation |
|---|---|---|---|---|
| Azure Key Vault provider for Secrets Store CSI | Long-running DaemonSet; Microsoft Entra ID and `*.vault.azure.net` | Image-baked Mariner distroless bundle by default | Secret mounts fail while local Unix-socket `/healthz` can remain healthy; validate the actual Entra/Key Vault operation | Host-mount a normalized node bundle, configure standard trust lookup to use it, and roll the provider DaemonSet |
| Azure Disk CSI node | Long-running DaemonSet; ARM/Compute/Disk APIs called by the node component | Requires image and client audit | Attach/mount reconciliation may fail; validate a safe component-owned Azure operation | Mount normalized node trust when system trust is intended and roll the node pod |
| Azure File CSI node | Long-running DaemonSet and mount helpers; ARM and Azure Storage | Mixed pod and helper trust requiring audit | New mounts or storage operations may fail | Update applicable pod/helper trust, roll the node pod, and avoid disrupting active mounts |
| Blob CSI node and Blobfuse child processes | Long-running DaemonSet plus process per mount; ARM and Azure Storage | Separate pod and Blobfuse trust sources | Plugin control path or existing mounts may fail independently | Roll the plugin for pod trust; use drain-safe recreation for affected mount processes |
| Azure cloud-node-manager | Long-running DaemonSet; ARM plus Kubernetes API | Public system roots for ARM must be separated from explicit cluster CA | ARM reconciliation may fail while Kubernetes API remains healthy | Mount normalized trust and roll only for the public ARM dependency; preserve cluster-CA configuration |
| Azure CNS, IPAM, CNI, and Cilium agents | Long-running agents plus short-lived CNI execution; feature-dependent networking endpoints and Kubernetes API | Mixed explicit cluster CA, local transport, and potentially public system roots | Only identified Microsoft HTTPS paths are in scope | Mount and roll only affected long-running agents; do not restart solely for Kubernetes API or local transport |
| Azure Monitor agents and Prometheus collectors | Long-running agents; Azure Monitor ingestion and control endpoints | Component-specific image trust requiring audit | Export/control requests fail while local collection can remain healthy | Add supported mounted trust or a refreshed image, then perform a controlled managed rollout |
| Azure Policy add-on | Long-running Deployment; Azure Policy service plus Kubernetes API | Public service trust and explicit cluster CA | Policy service reconciliation can fail independently of cluster API health | Update public system trust and roll; preserve explicit Kubernetes CA |
| Konnectivity agent | Long-running DaemonSet; AKS control-plane tunnel | Requires tunnel certificate and trust audit | Tunnel failure only belongs in scope if public system roots are used | Include only after confirming system-trust dependence; otherwise preserve explicit tunnel trust |
| `kube-proxy`, CSI registrar/liveness sidecars, add-on resizer, and kube-state-metrics | Long-running pods; primarily Kubernetes API or local sockets | Expected explicit cluster CA or local transport | No public-system-trust failure identified | No action without contrary evidence |
| CoreDNS and pause containers | DNS/local runtime behavior | No first-party HTTPS client path identified | No first-party CA failure identified | No action |

The cached VHD image list is useful for discovery but is not the source of truth
for affected pods. The actual add-on manifests, enabled cluster features, cloud
environment, and runtime configuration determine whether a component is
present and whether it contacts a first-party endpoint.

#### Key Vault provider example

The Linux provider image is based on
`mcr.microsoft.com/cbl-mariner/distroless/minimal:2.0`. Its standard DaemonSet
mounts `/var/run/secrets-store-csi-providers` at `/provider`, but it does not
mount `/etc/ssl/certs`, `/etc/pki`, or a node CA bundle. The Azure SDK clients
use the default Go HTTP transport.

| Provider behavior | RCV result |
|---|---|
| Node trust changes, but the pod has no node-bundle mount | Provider trust does not change |
| Provider restarts from the same image without a node-bundle mount | Provider still uses the image-baked bundle |
| A normalized node bundle is host-mounted | Updated bytes become visible inside the container |
| Mounted bundle changes while the process remains alive | In-memory Go roots can remain stale |
| Provider rolls after the mounted bundle changes | New process loads current mounted trust |
| Local Unix-socket `/healthz` succeeds | Proves local gRPC health only, not Entra ID or Key Vault TLS |

The provider's current liveness probe validates only its local Unix-socket gRPC
service. It does not validate Microsoft Entra ID or Key Vault TLS, so it can
remain healthy while secret mounts fail.

#### Managed-pod rollout contract

V1 should provide a normalized, read-only node CA bundle path so managed images
do not need to know whether the host is Ubuntu, Azure Linux, Mariner, Flatcar,
or another supported OS. Managed add-ons can mount that file at the path their
runtime expects.

After the bundle generation changes, AKS should roll only managed pods whose
owners have declared a system-trust dependency.

| Rollout requirement | Constraint |
|---|---|
| Availability | Bound by `maxUnavailable` and avoid simultaneous disruption across a node pool |
| Component safety | Preserve storage and networking recovery requirements |
| Trust targeting | Do not restart components that exclusively use the Kubernetes cluster CA |
| Completion | Report success only after a real first-party operation succeeds |

### 2.b Customer-owned pods

AKS cannot safely infer or mutate the trust behavior of arbitrary customer
workloads. Automatically mounting node trust or restarting every customer pod
would change security boundaries and can violate availability requirements.

| Customer trust pattern | Does node trust propagate automatically? | Required remediation |
|---|---|---|
| Image-baked OS CA bundle | No | Rebuild or update the image, then roll the workload |
| Node bundle mounted by hostPath | File changes are visible, but in-memory roots may remain stale | Restart the process or pod, or use an application-supported TLS reload |
| CA supplied through Secret or ConfigMap | No; lifecycle is controlled by that object | Update the object and trigger the application's supported reload or rollout |
| Java keystore, .NET store, NSS database, or other application-specific store | No | Update through the runtime-specific mechanism and recreate or reload the process |
| Private PEM configured through an SDK option | No | Update the private CA material and recreate or reload the client |
| Certificate or public-key pinning | No; system trust may be intentionally bypassed | Update pin policy explicitly; do not replace it with node trust |

AKS v1 responsibility is therefore to provide:

| AKS deliverable | Customer remediation enabled |
|---|---|
| Stable, read-only normalized node CA bundle path or supported projection | Customers can intentionally consume current node public trust |
| Mount examples for common Linux images and runtimes | Applications can direct standard trust lookup to the mounted bundle |
| Node condition, event, annotation, or observable trust-generation signal | Customer automation can trigger a controlled rollout |
| Guidance separating file projection from process reload | Customers avoid assuming visible changed bytes refresh cached trust |
| Deployment, StatefulSet, and DaemonSet rollout examples | Workload owners can preserve their availability requirements |
| Explicit CA-removal guidance | Customers stop trusting removed roots rather than retaining stale in-memory trust |

Customer responsibility is to:

| Customer action | Required result |
|---|---|
| Select the intended trust source | Node public trust is not confused with private or cluster-specific trust |
| Mount or copy node trust only when desired | Security boundaries remain explicit |
| Configure language- or application-specific trust | Runtime uses the intended bundle or store |
| Implement hot reload or workload restart | Long-running clients load the updated roots |
| Honor disruption and stateful recovery requirements | Trust remediation does not create uncontrolled availability loss |
| Validate the actual first-party TLS operation | Local liveness is not mistaken for endpoint health |

AKS should not automatically restart customer pods in v1. A controlled node
drain and reboot may recreate them as a side effect of node remediation, but it
is a last-resort node recovery action rather than the customer-pod trust
contract.

## V1 deliverables

| Scope | V1 deliverable | Remediation outcome |
|---|---|---|
| Node | Deliver and validate AgentBaker PR #9432 | Containerd restarts only when the effective native system bundle changes |
| Node | Complete the host-package audit across supported Linux VHD variants | Every long-running first-party TLS client has a targeted reload/restart or documented exclusion |
| Node | Add safe, side-effect-free component probes | Remediation is based on actual stale trust rather than broad process restarts |
| Node | Define external, rate-limited cordon/drain/reboot escalation | Node reboot remains a bounded fallback |
| Node | Validate CA addition and removal independently | Availability and distrust/security behavior are covered |
| AKS-owned pods | Inventory enabled managed add-ons, endpoints, trust sources, and TLS runtimes | Each owner selects image, mounted-system, explicit, or private trust intentionally |
| AKS-owned pods | Provide a normalized node CA bundle mount where system trust is intended | Managed pods receive current node public trust |
| AKS-owned pods | Define targeted rolling restart or supported hot reload | Long-running clients recreate or reload after trust changes |
| AKS-owned pods | Validate the relevant first-party operation | Local liveness cannot mask endpoint TLS failure |
| Customer-owned pods | Publish node trust-consumption and generation contracts | Customers can opt in and automate controlled rollouts |
| Customer-owned pods | Document restart and hot-reload patterns without mutating workloads | Application-specific remediation remains customer-controlled |

## Out of scope for v1

- Automatically injecting node trust into all pods.
- Automatically restarting all customer workloads.
- Treating every cached VHD image as an affected running component.
- Replacing explicit Kubernetes cluster CAs or customer-private trust stores.
- Disabling hostname, chain, expiration, or revocation validation.
- Restarting a component solely because it is written in Go.
- Direct node reboot initiated by NPD.
- Windows process and pod trust propagation without a dedicated Windows audit.

## Exit criteria

RCV v1 is ready for rollout when:

1. unchanged node trust causes no containerd restart;
2. changed node trust produces a new healthy containerd invocation while
   existing workload containers survive;
3. the host-package audit has an owner and disposition for every long-running
   first-party TLS client;
4. each in-scope AKS-managed pod either consumes current node trust and reloads
   it or has a documented reason for using another trust source;
5. remediation is bounded and cannot loop on one CA generation;
6. addition and removal scenarios are validated; and
7. customer-owned workloads have a stable, documented opt-in path without
   automatic mutation or restart.
