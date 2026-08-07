version = 2
oom_score = -999{{if getHasDataDir .KubeletConfig}}
root = "{{.KubeletConfig.GetContainerDataDir}}"{{- end}}
{{- if .GetIsKata }}
[plugins."io.containerd.snapshotter.v1.erofs"]
  default_size = "10G"
  enable_fsverity = false
  ovl_mount_options = []
  max_unmerged_layers = 1

[plugins."io.containerd.service.v1.diff-service"]
  default = ["erofs", "walking"]

[plugins."io.containerd.differ.v1.erofs"]
  mkfs_options = ["-T0", "--mkfs-time", "--sort=none"]
  enable_tar_index = false
{{- end}}
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = "{{ .KubeBinaryConfig.GetPodInfraContainerImageUrl }}"
  [plugins."io.containerd.grpc.v1.cri".containerd]
    {{- if .GetIsKata }}
    disable_snapshot_annotations = false
    snapshotter = "overlayfs"
    {{- end}}
    {{- if .GetEnableArtifactStreaming }}
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    {{- end}}
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      {{- if .GetNeedsCgroupv2 }}
      SystemdCgroup = true
      {{- end}}
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  {{- if getEnsureNoDupePromiscuousBridge .GetNetworkConfig }}
  [plugins."io.containerd.grpc.v1.cri".cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
  {{- end}}
  {{- if isKubernetesVersionGe .GetKubernetesVersion "1.22.0"}}
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  {{- end}}
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
{{- if .GetEnableArtifactStreaming }}
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
{{- end}}
{{- if .GetIsKata }}
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
  snapshotter = "overlayfs"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli]
  runtime_type = "io.containerd.runc.v1"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.katacli.options]
  NoPivotRoot = false
  NoNewKeyring = false
  ShimCgroup = ""
  IoUid = 0
  IoGid = 0
  BinaryName = "/usr/bin/kata-runtime"
  Root = ""
  CriuPath = ""
  SystemdCgroup = false
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-preview]
  runtime_type = "io.containerd.kata.v2"
  privileged_without_host_devices = true
  snapshotter = "erofs"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-preview.options]
    ConfigPath = "/usr/share/defaults/kata-containers/configuration-clh-templating.toml"
[proxy_plugins]
  [proxy_plugins.tardev]
    type = "snapshot"
    address = "/run/containerd/tardev-snapshotter.sock"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-cc]
  snapshotter = "tardev"
  runtime_type = "io.containerd.kata-cc.v2"
  privileged_without_host_devices = true
  pod_annotations = ["io.katacontainers.*"]
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.kata-cc.options]
    ConfigPath = "/opt/confidential-containers/share/defaults/kata-containers/configuration-clh-snp.toml"
{{- end}}
