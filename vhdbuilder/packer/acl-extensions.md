# ACL provisioning extensions

The ACL amd64 and arm64 Packer templates capture provisioning writes to
`/opt` and `/etc` using systemd's persistent mutable extension overlays.
After CIS, cleanup, and waagent deprovisioning, the writable layers become
`/var/lib/extensions/aks-opt.raw` and `/var/lib/confexts/aks-etc.raw`.
The provisioning payload is stored in the images, not copied back into
the base directories. Exceptions are boot-activation drop-ins and the
empty deprovisioned `/etc/machine-id`: PID 1 initializes machine identity
before confext activation, so machine-id must not be in the confext.

The base ACL image must provide systemd >= 256, `systemd-confext`, and
`mkfs.ext4`. Capture fails if these are absent, preexisting mutable layers
are nonempty, or probe writes do not reach the capture layers. This is an
experimental build path; validate against the selected ACL SIG base before
publishing images.

The raw images are ext4 filesystems. `mkfs.ext4 -d` preserves ownership,
modes, symlinks, xattrs, whiteout devices, and opaque-directory attributes.
These are necessary to retain modifications and deletions, not just additions.
Extension-release metadata uses the base OS identity and compatibility level.

Boot activation uses base `/etc/systemd/system` drop-ins, not drop-ins
inside the confext. Confext activates before sysext because it supplies
`/etc/extensions` links. Both services use persistent mutable mode, and ACL
CSE sysext refreshes retain that mode. Confext uses `--noexec=false` to
preserve executable configuration-tree behavior. Runtime node-specific
writes go to `/var/lib/extensions.mutable/{opt,etc}`.

Existing kubelet/GPU sysext downloads use `/var/lib/aks-sysext-cache`
instead of `/opt`: the images must be discoverable before the `/opt`
extension is mounted. Ignition-created bootstrap files predate capture and
remain in the base image. Files outside `/opt` and `/etc` are not captured.

The existing VHD-to-COSI converter is unchanged. Verify that the generated
COSI contains the extension images and that its deployment restores the
required `/var` content. In particular, an A/B update that only replaces
`/usr` will not replace extensions or runtime layers stored in `/var`;
extension distribution and mutable-state reconciliation for that update
path remain separate work.

## Validation

Run ShellSpec for `spec/vhdbuilder/packer/acl_extensions_spec.sh`, the ACL
install spec, and the COSI conversion spec. Running the extension spec as
root also exercises real OverlayFS-to-ext4-to-OverlayFS round trips inside
a private mount namespace, including deletions and opaque directories.
This does not exercise systemd merge/dissection or substitute for an ACL VM.

Before release, build both ACL architectures (including FIPS), boot VHD
and COSI deployments, verify extension activation before node provisioning,
test writable `/opt` and `/etc`, refresh kubelet/GPU sysexts, reboot, and
check deprovisioned account/machine state. Check an actual A/B update
separately; persistence in `/var` is not evidence of successful servicing.
