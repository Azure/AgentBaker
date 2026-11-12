# Public Kata default versions

Azure Linux 3 Gen2 Kata VHD release notes contain an explicit `Kata Default
Versions` block. It describes the host kernel, UVM kernel, and Kata runtime
shipped by the VHD build, not later customer updates. It does not describe
standard Azure Linux or confidential containers (Kata-CC).

The Kata UVM pipeline (definition 417429 in aks-rp) publishes
`kata-uvm-kernel-version.txt` alongside `kata-containers.img`. The file contains
one RPM version-release, for example `6.18.52.mshv1-2.azl3`, followed by a newline.
The existing `kata-containers-version.txt` remains unchanged and continues to
validate the host/runtime and UVM tools package compatibility.

AgentBaker downloads both files from the same artifact, validates them, and
passes the kernel value through Packer. After the host has rebooted,
`write-kata-release-notes.sh` records the booted host kernel and installed runtime
alongside the supplied UVM kernel value. The resulting block is part of the
normal public release note under `vhdbuilder/release-notes/AKSAzureLinuxV3/gen2kata`.
It intentionally does not publish the upstream UVM build ID.

Merge and successfully run the UVM metadata producer before deploying this
consumer. Old artifacts without the kernel metadata must be rebuilt; the
consumer fails rather than guessing a default from independently installed RPMs.

The release tracker reads this block from public GitHub release notes and shows
it in the existing Azure Linux section for the selected VHD version. Older
notes without the block remain accessible through the source link.
