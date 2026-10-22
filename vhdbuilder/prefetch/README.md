# Prefetch

Provides functionality to generate scripts used for prefetch optimization.

## Final AIB deprovisioning

The optimization template installs `/tmp/DeprovisioningScript.sh`, AIB's supported
Linux deprovisioning override. Customization only installs the hook; it does not
run deprovisioning. The hook and the existing WALinuxAgent installer are downloaded
from the build's exact AgentBaker commit (`GIT_VERSION`), not a moving branch.

The hook retains AIB's `waagent -force -deprovision+user`, clears shell history
size, and syncs writes. After generalization it reruns the same installer used
after Packer deprovisioning: install the `components.json` WALinuxAgent version,
prune competing agent code directories, and retain local agent updates without
network auto-updates. Flatcar, Azure Container Linux and OSGuard retain their
existing agent-install exclusions. No VM identity or extension state is restored.

Only Ubuntu 26.04 builds with the `cvm` feature remove systemd NvPCR anchor
credentials before capture, including persistent, runtime and standard
ESP/XBOOTLDR credential copies. Other TPM state, disk-encryption material and
unrelated credentials are not removed. Read-only boot-imported credentials under
`/run/credentials/@encrypted` are RAM-only and disappear at shutdown; their boot
sources are removed instead. Both SNP and TDX are covered.

Validate a newly optimized image on a fresh VM and after a reboot: the running
WALinuxAgent handler must match `components.json`, and both TPM setup services
must run successfully without anchor-secret integrity errors. This also checks
the final hook's ordering relative to AIB's optimization boots.