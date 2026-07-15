{{- $isContainerdConfigV4 := isContainerdVersionGe .GetContainerdConfig "2.3.0" -}}
{{- $imagesPlugin := "io.containerd.grpc.v1.cri" -}}
{{- $runtimePlugin := "io.containerd.grpc.v1.cri" -}}
{{- if $isContainerdConfigV4 -}}
{{- $imagesPlugin = "io.containerd.cri.v1.images" -}}
{{- $runtimePlugin = "io.containerd.cri.v1.runtime" -}}
{{- end -}}
version = {{if $isContainerdConfigV4}}4{{else}}2{{end}}
oom_score = -999{{if getHasDataDir .KubeletConfig}}
root = "{{.KubeletConfig.GetContainerDataDir}}"{{- end}}
{{- if $isContainerdConfigV4 }}
[plugins."{{$imagesPlugin}}".pinned_images]
  sandbox = "{{ .KubeBinaryConfig.GetPodInfraContainerImageUrl }}"
{{- else }}
[plugins."{{$imagesPlugin}}"]
  sandbox_image = "{{ .KubeBinaryConfig.GetPodInfraContainerImageUrl }}"
{{- end }}
  [plugins."{{$runtimePlugin}}".containerd]
    {{- if .GetIsKata }}
    disable_snapshot_annotations = false
    snapshotter = "overlayfs"
    {{- end}}
    {{- if .GetEnableArtifactStreaming }}
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    {{- end}}
    default_runtime_name = "runc"
    [plugins."{{$runtimePlugin}}".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."{{$runtimePlugin}}".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      {{- if .GetNeedsCgroupv2 }}
      SystemdCgroup = true
      {{- end}}
    [plugins."{{$runtimePlugin}}".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."{{$runtimePlugin}}".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  {{- if getEnsureNoDupePromiscuousBridge .GetNetworkConfig }}
  [plugins."{{$runtimePlugin}}".cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
  {{- end}}
  {{- if isKubernetesVersionGe .GetKubernetesVersion "1.22.0"}}
  [plugins."{{$imagesPlugin}}".registry]
    config_path = "/etc/containerd/certs.d"
  {{- end}}
  [plugins."{{$imagesPlugin}}".registry.headers]
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
[plugins."{{$runtimePlugin}}".containerd.runtimes.kata]
  runtime_type = "io.containerd.kata.v2"
[plugins."{{$runtimePlugin}}".containerd.runtimes.katacli]
  runtime_type = "io.containerd.runc.v1"
[plugins."{{$runtimePlugin}}".containerd.runtimes.katacli.options]
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
    ConfigPath = "/usr/share/defaults/kata-containers/configuration-clh-preview.toml"
[proxy_plugins]
  [proxy_plugins.tardev]
    type = "snapshot"
    address = "/run/containerd/tardev-snapshotter.sock"
[plugins."{{$runtimePlugin}}".containerd.runtimes.kata-cc]
  snapshotter = "tardev"
  runtime_type = "io.containerd.kata-cc.v2"
  privileged_without_host_devices = true
  pod_annotations = ["io.katacontainers.*"]
  [plugins."{{$runtimePlugin}}".containerd.runtimes.kata-cc.options]
    ConfigPath = "/opt/confidential-containers/share/defaults/kata-containers/configuration-clh-snp.toml"
{{- end}}
