# ACL provisioning extensions

The ACL amd64 and arm64 Packer templates capture provisioning writes to
`/opt` and `/etc` in explicit writable OverlayFS mounts. After CIS, cleanup,
and waagent deprovisioning, their upper directories become
`/var/lib/aks-extensions/aks-opt.raw` and `aks-etc.raw`.
The provisioning payload is stored in these images, not copied back into
the base directories. Ignition-created files and the activation service
predate capture and remain in the base. Machine-id is excluded from the
confext and reset to an empty base file for PID 1 to initialize.

## systemd 255 compatibility

The base image must provide systemd >= 255, OverlayFS, loop devices,
`mkfs.ext4`, and the util-linux mount/flock tools. No systemd 256 mutable
extension options or `systemd-confext` executable are required.

Systemd 255's native extension mounts are read-only. Instead,
`aks-extensions.service` mounts the raw images read-only, validates their
OS identity/version or compatibility level, then builds these writable views:

```
/opt = runtime upper + aks-opt.raw:/opt + original /opt
/etc = runtime upper + aks-etc.raw:/etc + original /etc
```

Runtime uppers and workdirs live in `/var/lib/aks-extensions.mutable/`.
They survive reactivation/reboot; runtime writes do not modify the images.
Images are plain ext4 filesystems with standard sysext/confext release metadata.
`mkfs.ext4 -d` preserves ownership, modes, symlinks, xattrs, whiteout devices,
and opaque-directory attributes, retaining deletions as well as additions.

The activation script and service are installed before capture. The service
activates `/etc` before `/opt`, after local filesystems and before sysinit,
native sysext, sysusers, tmpfiles, and cloud-init-local. It reloads the
manager after activation so captured units become visible.

The images intentionally live outside native extension discovery directories.
Native `systemd-sysext` manages **only `/usr`**: both its service drop-in and
the ACL CSE environment set `SYSTEMD_SYSEXT_HIERARCHIES=/usr`. ACL kubelet/GPU
extensions must put their runtime payload in `/usr`; external extensions
requiring native `/opt` merging are not supported by this layout.
Manual native refresh commands must use the same environment setting.
Non-ACL provisioning is unchanged.

Kubelet/GPU downloads use `/var/lib/aks-sysext-cache`, independently of the
captured `/opt`. Files outside `/opt` and `/etc` are not captured.

The existing VHD-to-COSI conversion is unchanged. Deployment must restore the
required `/var` content. An A/B update replacing only `/usr` does not replace
these images or reconcile runtime uppers; their distribution and servicing
remain separate work.

## Pipeline artifact

Successful ACL Packer jobs publish `acl-extensions-${{ parameters.artifactName }}`
with `aks-opt.raw`, `aks-etc.raw`, `aks-opt.files.txt`, and `aks-etc.files.txt`.
The suffix distinguishes architecture and FIPS/Trusted Launch variants.
ImageCustomizer and non-ACL jobs do not publish this artifact.

Listings mount each finalized image read-only with journal replay disabled and
run `find -ls`. They include every image entry, metadata, directories, symlink
targets, whiteouts, permissions, ownership, size/device number, and timestamps.
They describe the image, not its merged filesystem view.

Both Packer templates download all four files immediately after finalization,
using the existing authenticated SSH connection after deprovisioning. No reboot
is inserted. Missing/empty exports fail publication.

## Validation

Run the ACL extension, extension lifecycle, ACL install, and CVM ShellSpecs;
also run the COSI conversion spec where available. Root extension tests use
private mount namespaces and exercise actual OverlayFS/ext4 packaging,
whiteouts, opaque directories, captured versus base files, machine-id
exclusion, writable persistent uppers, reactivation, and failure cleanup.
On systemd >= 255, the lifecycle test also merges a native `/usr` extension
and verifies that a refresh from inside `/opt` leaves its mount writable.

Before release, run the complete ACL Packer pipeline on both architectures,
including FIPS, boot VHD/COSI deployments, verify activation and native
kubelet/GPU refreshes, reboot, and check deprovisioned account/machine state.
Validate actual A/B servicing separately; persistence in `/var` alone is not
evidence that an update distributes the new extensions correctly.
