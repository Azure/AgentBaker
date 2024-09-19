#!/bin/sh

scp windows:/AzureData/provision.complete .
scp windows:/AzureData/CustomDataSetupScript.log .
scp windows:/AzureData/CustomData.bin .
scp windows:/k/containerd.log .
scp windows:/k/containerd.err.log .
scp windows:/k/windowsnodereset.log .
scp windows:/k/kubelet.log .
scp windows:/k/kubelet.err.log .
scp windows:/k/csi-proxy.log .
scp windows:/k/csi-proxy.err.log .
scp windows:/k/kubeproxy.log .
scp windows:/k/kubeproxy.err.log .
