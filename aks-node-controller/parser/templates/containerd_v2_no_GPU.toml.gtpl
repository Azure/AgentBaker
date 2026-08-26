{{- $isV4 := isContainerdVersionGe .GetContainerdConfig "2.3.0" -}}
version = {{if $isV4}}4{{else}}2{{end}}
oom_score = -999{{if getHasDataDir .KubeletConfig}}
root = "{{.KubeletConfig.GetContainerDataDir}}"{{- end}}
{{- if .GetIsKata }}
[plugins."io.containerd.snapshotter.v1.erofs"]
  default_size = "10G"
  enable_fsverity = false
  ovl_mount_options = []

[plugins."io.containerd.service.v1.diff-service"]
  default = ["erofs", "walking"]

[plugins."io.containerd.differ.v1.erofs"]
  mkfs_options = ["-T0", "--mkfs-time", "--sort=none"]
  enable_tar_index = false
{{- end}}
[plugins."io.containerd.cri.v1.images"]
{{- if .GetEnableArtifactStreaming }}
  snapshotter = "overlaybd"
  disable_snapshot_annotations = false
{{- else if .GetIsKata }}
  disable_snapshot_annotations = false
{{- end}}
  [plugins."io.containerd.cri.v1.images".pinned_images]
    sandbox = "{{ .KubeBinaryConfig.GetPodInfraContainerImageUrl }}"
  {{- if isKubernetesVersionGe .GetKubernetesVersion "1.22.0"}}
  [plugins."io.containerd.cri.v1.images".registry]
    config_path = "/etc/containerd/certs.d"
  {{- end}}
  [plugins."io.containerd.cri.v1.images".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[plugins."io.containerd.cri.v1.runtime".containerd]
    default_runtime_name = "runc"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  {{- if getEnsureNoDupePromiscuousBridge .GetNetworkConfig }}
  [plugins."io.containerd.cri.v1.runtime".cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
  {{- end}}
[metrics]
  address = "0.0.0.0:10257"
{{- if .GetEnableArtifactStreaming }}
[proxy_plugins.overlaybd]
  type = "snapshot"
  address = "/run/overlaybd-snapshotter/overlaybd.sock"
{{- end}}
{{- if .GetIsKata }}
[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
  privileged_without_host_devices = true
  snapshotter = "overlayfs"
  [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata.options]
    ConfigPath = "/usr/share/defaults/kata-containers/configuration.toml"
[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-preview]
  runtime_type = "io.containerd.kata.v2"
  privileged_without_host_devices = true
  snapshotter = "erofs"
  [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-preview.options]
    ConfigPath = "/usr/share/defaults/kata-containers/configuration-clh-preview.toml"
[proxy_plugins.tardev]
  type = "snapshot"
  address = "/run/containerd/tardev-snapshotter.sock"
[plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-cc]
  snapshotter = "tardev"
  runtime_type = "io.containerd.kata-cc.v2"
  privileged_without_host_devices = true
  pod_annotations = ["io.katacontainers.*"]
  [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-cc.options]
    ConfigPath = "/opt/confidential-containers/share/defaults/kata-containers/configuration-clh-snp.toml"
{{- end}}
