[Unit]
Description=AKS Local DNS Slice
DefaultDependencies=no
Before=slices.target
Requires=system.slice
After=system.slice

[Slice]
MemoryMax=128M