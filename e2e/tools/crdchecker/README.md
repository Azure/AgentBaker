# crd-checker

This tool can be used to verify whether a built VHD meets or exceeds the required versions of a provided CRD JSON file. It takes flags indicating files containing the CRD and the release notes of a given AgentBaker VHD.

Typical usage:

```bash
$ python3 hack/crd-checker/crdchecker.py --hpc-crd-file bom-1.3.json --vhd-relnotes-file release-notes.txt
Installed version for nvidia-imex is fresher than CRD: installed has 580.95.05-1, CRD has 580.82.07
Installed version for NVIDIA aka libnvidia-common-580 is fresher than CRD: installed has 580.95.05, CRD has 580.82.07
Installed version for CUDA aka cuda-driver-dev-13-0 is fresher than CRD: installed has 13.0.96-1, CRD has 13.0.88
Installed version for DCGM aka datacenter-gpu-manager-4-core is fresher than CRD: installed has 4.4.1-1, CRD has 4.4.1
Installed version for DCGM aka datacenter-gpu-manager-4-proprietary is fresher than CRD: installed has 4.4.1-1, CRD has 4.4.1
Installed version for DCGM aka datacenter-gpu-manager-exporter is fresher than CRD: installed has 4.5.2-1, CRD has 4.4.1
Installed version for linux-azure-nvidia is fresher than CRD: installed has 6.8.0-1025.27, CRD has 6.8.0-1024
Installed version for mlnx-ofed-kernel-dkms is fresher than CRD: installed has 25.07.0.9.7.0.214.1-1, CRD has 25.07-0.9.7.0.214
CRD check passed!
```

Note that we log when the VHD contains a version *newer* than the CRD BOM.
