# Prefetch

Provides functionality to generate scripts used for prefetch optimization.

After Image Builder finishes, its original post-prefetch VHD is used concurrently
for generalized CVM gallery publication and delivery to the final storage account
(via the mutable staging container and then the immutable VHD container). Gallery
versions are created directly from that VHD, without an intermediate managed image,
in the source storage account's region. Replication includes East US, West Europe,
and the source region when different. The managed image used as Image Builder's
input for TL/CVM builds is unchanged.

The prefetch task waits for both storage delivery and gallery replication to finish
before setting `PREFETCH_DELIVERY_COMPLETE=True`. The cleanup task requires this
signal and deletes the Image Builder template before its resource group, releasing
the service-managed staging storage and original VHD. On failure, cleanup is
blocked; retries reuse the original output and do not skip missing gallery
publication just because the immutable VHD already exists.