# Prefetch

Provides functionality to generate scripts used for prefetch optimization.

After Image Builder finishes, its original post-prefetch VHD is used concurrently
for generalized CVM gallery publication and delivery to the final storage account
(via the mutable staging container and then the immutable VHD container). Gallery
versions are created directly from that VHD, without an intermediate managed image,
in the source storage account's region. Replication includes East US, West Europe,
and the source region when different. The managed image used as Image Builder's
input for TL/CVM builds is unchanged.

`copy_optimized_vhd` and `copy_optimized_vhd_to_packer_gallery` receive the same
Image Builder artifact URI and run in parallel. The prefetch task waits for both
storage delivery and gallery replication, even if either fails. Storage delivery
failure is fatal; gallery publication failure emits a pipeline warning but does
not fail the task. Cleanup can run once both operations finish and storage delivery
has succeeded.
The storage function deletes its own staging copy after immutable delivery;
Image Builder's original VHD remains available until the later cleanup task.

The existing retry behavior is unchanged: an already-completed immutable VHD
causes an early exit without checking gallery publication.

The publisher uses Azure CLI's built-in wait for provisioning and replication.
It does not inspect, delete, or recover failed image versions before publication;
Azure CLI failures propagate to the prefetch task.