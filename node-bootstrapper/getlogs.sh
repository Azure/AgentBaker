#!/bin/sh

mkdir -p logs

scp windows:/AzureData/provision.complete logs/
scp windows:/AzureData/CustomDataSetupScript.log logs/
scp windows:/AzureData/CustomData.bin logs/
scp windows:/k/containerd.log logs/
scp windows:/k/containerd.err.log logs/
scp windows:/k/windowsnodereset.log logs/
scp windows:/k/kubelet.log logs/
scp windows:/k/kubelet.err.log logs/
scp windows:/k/csi-proxy.log logs/
scp windows:/k/csi-proxy.err.log logs/
scp windows:/k/kubeproxy.log logs/
scp windows:/k/kubeproxy.err.log logs/
