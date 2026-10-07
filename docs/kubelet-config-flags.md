# Linux kubelet renderer omission contract

This is default-off renderer support, not Kubernetes-version policy or config-mode activation. An upstream producer must choose the supported node version, effective settings and compatible provisioning artifacts before using it. Existing callers that do not send the new instruction retain their existing kubelet output, even when config-file mode is already enabled.

## Request

Set only `EnabledFeatures["KUBELET_FLAGS_TO_OMIT"]` in NBC, or `enabled_features["KUBELET_FLAGS_TO_OMIT"]` in ANC configuration. Its value is an **unpadded standard-base64** encoded JSON string array of full flag names, at most 1024 encoded bytes and 16 entries. Go producers can use `base64.RawStdEncoding.EncodeToString`. For example, encode `["--enable-server","--runtime-request-timeout"]`. The existing launcher feature-file reader strips trailing `=` padding; unpadded encoding works without changing that historical reader. The Linux renderer also accepts/restores padding on valid standard-base64 inputs.

The producer selects the subset; no flags are selected automatically. Unknown names are ignored. Malformed/oversized lists, missing or nonmatching fields, config-off and an alternative `--config`/`--config-dir` in custom flags preserve the existing arguments. Empty/absent instructions do nothing, except removing this renderer's own previous override during rollback.

## Replacement checks

The loaded `/etc/default/kubeletconfig.json` and its standard config drop-in must exist. Each requested name is omitted only for this explicit matching value:

| Requested local argument | Required config value | Versioned upstream deprecation evidence |
|---|---|---|
| `--enable-server` | `enableServer: true` (missing/false is not a match) | [v1.19.0](https://github.com/kubernetes/kubernetes/blob/v1.19.0/cmd/kubelet/app/options/options.go#L403), [v1.37.0](https://github.com/kubernetes/kubernetes/blob/v1.37.0/cmd/kubelet/app/options/options.go#L364) |
| `--volume-plugin-dir` | `volumePluginDir: /etc/kubernetes/volumeplugins` | [v1.19.0](https://github.com/kubernetes/kubernetes/blob/v1.19.0/cmd/kubelet/app/options/options.go#L470), [v1.37.0](https://github.com/kubernetes/kubernetes/blob/v1.37.0/cmd/kubelet/app/options/options.go#L436) |
| `--cgroup-driver` | `cgroupDriver: systemd`, `NEEDS_CGROUPV2=true`, recognized cgroup drop-in | [v1.10.0](https://github.com/kubernetes/kubernetes/blob/v1.10.0/cmd/kubelet/app/options/options.go#L518), [v1.37.0](https://github.com/kubernetes/kubernetes/blob/v1.37.0/cmd/kubelet/app/options/options.go#L452) |
| `--runtime-request-timeout` | `runtimeRequestTimeout: 15m` | [v1.10.0](https://github.com/kubernetes/kubernetes/blob/v1.10.0/cmd/kubelet/app/options/options.go#L522), [v1.37.0](https://github.com/kubernetes/kubernetes/blob/v1.37.0/cmd/kubelet/app/options/options.go#L459) |
| `--container-runtime-endpoint` | `containerRuntimeEndpoint: unix:///run/containerd/containerd.sock` | [v1.27.0](https://github.com/kubernetes/kubernetes/blob/v1.27.0/cmd/kubelet/app/options/options.go#L372), [v1.37.0](https://github.com/kubernetes/kubernetes/blob/v1.37.0/cmd/kubelet/app/options/options.go#L379) |

These registrations belong to the [deprecated config-flag family](https://github.com/kubernetes/kubernetes/blob/v1.37.0/cmd/kubelet/app/options/options.go#L334). Deprecation evidence does not establish availability or validation of a future binary.

The renderer never rewrites `KUBELET_FLAGS`, including duplicate/custom arguments; they remain last and retain their precedence. Alternate duration spellings such as `15m0s` conservatively retain the timeout flag. Nonmatching fields do not authorize a behavior change. Reserved-cgroup/hardening policy, runtime-cgroup selection, TLS/bootstrap, IMDS restriction, networking and sandbox-image configuration are unchanged.

## Provisioning and rollback

`ensureKubelet` reconciles `11-kubelet-config-flags.conf` after the existing final runtime writer and before daemon-reload/start. This is in real-node `nodePrep`, including PIS nodes that skip `basePrep`. The globally baked service and original drop-ins stay unchanged. Reconciliation is idempotent; dropping the instruction or disabling config mode removes only this managed override.

All omissions require the recognized current `ExecStart` layout and no other `ExecStart` override in the kubelet drop-in directory, so the standard config environment is actually used. Unknown service layouts retain all arguments. Runtime-cgroup output is taken from the existing final writer, including its hardened slice selection. The cgroup driver is cleared only for the recognized original cgroupv2 drop-in. This prerequisite is not a general custom-systemd-unit migration.

## Supported delivery paths and prerequisites

- **Legacy CSE:** the NBC map's one reserved omission key is explicitly injected into the generated CSE child environment. No generic feature-map forwarding is introduced. The legacy config generator does not currently accept a producer-owned complete configuration; missing replacement values therefore retain the local flags. The omission instruction is not an authoritative-config transport.
- **Scriptless NBC:** the generated NBC command carries the same explicit assignment; the existing boothook feature-file transport is retained.
- **Typed ANC:** the existing `enabled_features` boothook writes the key; the launcher exports it and `BuildCSECmd` inherits it. The producer must actually populate that key and provide the explicit config values. Schema/map presence alone is not producer activation.

No producer activation or schema changes are included. Future producers must limit the new instruction to their explicitly selected stable node Kubernetes >=1.38 cohort, preserve older/prerelease/default/override behavior, and verify consumer plus VHD/CSE compatibility. In particular, a newer library does not replace an old VHD's `provision_configs_kubelet.sh`. Unsupported old consumers/images must retain fallback arguments; a compatible script-delivery/release path and known service layout are prerequisites. Rollback to an older renderer must not leave this newer renderer's managed override behind.

Local tests cover rendering, transport, boundaries, fallback, PIS ordering and reversible output. They do not prove matching future Kubernetes binary acceptance, node readiness, networking/security equivalence or all historical VHD combinations. Linux support does not imply Windows parity.
