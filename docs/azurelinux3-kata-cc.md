# AZL3 Kata-CC image contract

The opt-in `buildAzureLinuxV3gen2katacc` release job consumes a specific successful
run (`kataCCSigningBuildId`) of signing pipeline **318279**, artifact
`drop_container_job_sign_uvm_azl3`. It does not take an unsigned artifact from the
UVM build pipeline or silently select a different signing run.

| Interface | Value |
| --- | --- |
| Distro | `AKSAzureLinuxV3Gen2KataCC` / `aks-azurelinux-v3-gen2-kata-cc` |
| SIG gallery/definition | `AKSAzureLinux/V3kataccgen2` |
| Build artifact | `azurelinuxv3-gen2-kata-cc` |
| OS_VERSION / Hyper-V generation | `V3katacc` / `V2` |
| Kubernetes RuntimeClass / handler | `kata-cc-isolation` / `kata-cc` |
| Scriptless workload intent | `WORKLOAD_RUNTIME_KATA_CC_ISOLATION` |

`prepare-kata-cc-artifacts.sh` normalizes the current signing artifact layout,
checks required files, and records the producer run. A CC-only generated Packer
template uploads the payload directory. `installKataCCPayload` stages the guest,
IGVM and endorsement files at the existing confidential-container package path.
The ordinary AZL3 Kata image/job retains its unsigned-build input and identity.

The image uses the existing `kata-packages-host` installation and current
`io.containerd.kata-cc.v2`, tardev and `configuration-clh-snp.toml` runtime
interfaces. Both scriptful and scriptless containerd v1/v2 paths can render a
Kata-CC handler. This is real integration wiring, **not evidence that current
AZL3 packages can run a confidential workload**. No OpenVMM package paths or
future signing formats are assumed. Missing producer files fail the build at
the point of use; there is no unconditional readiness blocker.

When refreshed packages arrive, update the signing job, package installation,
payload staging and runtime templates in AgentBaker as necessary. Preserve the
RP-facing distro and workload intent above. Publish a **new version** under
`V3kataccgen2`, and release matching AgentBakerSvc/node-controller bits; do not
overwrite an existing image version. Prefer matching host/guest package versions
from the producer once it provides that metadata; the current producer does not
establish that version-pinning contract.

The new definition uses AgentBaker's existing unpublished-definition exclusion
in `GetMaintainedLinuxSIGImageConfigMap`. Remove only its exclusion after the
first VHD is published and replicated, before enabling the default-off RP
`enable-azurelinux-v3-kata-cc` toggle. Validate signed-image provisioning and CC
workloads before activation. Merely signing an endorsement does not prove host
and guest compatibility or workload functionality.

RP must consume the coordinated AgentBaker and aks-node-controller commit (or a
release containing it) in its initial integration. This ensures local fallback
image lookup and both scriptless converters know the same identity and enum.
Future payload changes then stay behind those interfaces. Existing pools need
an image upgrade/reimage to receive a corrected image; publication alone does
not change running nodes.
