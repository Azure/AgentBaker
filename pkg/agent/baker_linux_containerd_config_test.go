package agent

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assert generated customData and cseCmd", func() {
	DescribeTable("Generated customData and CSE for Linux + Ubuntu", CustomDataCSECommandTestTemplate,
		Entry("AKSUbuntu2404 containerd v2 CRI plugin config should have rename containerd runtime name", "AKSUbuntu2404+Teleport", ">=1.32.x",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2404
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.32.0"
				// to have snapshotter features
				config.EnableACRTeleportPlugin = true
			}, func(o *nodeBootstrappingOutput) {
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedVersion := `version = 3`
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedVersion))

				expectedContainerdV2CriConfig := `
[plugins."io.containerd.cri.v1.images".pinned_images]
  sandbox = ""
`
				deprecatedContainerdV1CriConfig := `
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
`
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedContainerdV2CriConfig))
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(deprecatedContainerdV1CriConfig))

				expectedSnapshotterConfig := `
[plugins."io.containerd.cri.v1.images"]
  snapshotter = "teleportd"
  disable_snapshot_annotations = false
`
				deprecatedSnapshotterConfig := `
[plugins."io.containerd.grpc.v1.cri".containerd]
  snapshotter = "teleportd"
  disable_snapshot_annotations = false
`
				Expect(expectedSnapshotterConfig).NotTo(Equal(deprecatedSnapshotterConfig))
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedSnapshotterConfig))
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(deprecatedSnapshotterConfig))

				expectedRuncConfig := `
[plugins."io.containerd.cri.v1.runtime".containerd]
  default_runtime_name = "runc"
  [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.runc]
    runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
`
				deprecatedRuncConfig := `
[plugins."io.containerd.grpc.v1.cri".containerd]
  default_runtime_name = "runc"
  [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
    runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
`
				Expect(expectedRuncConfig).NotTo(Equal(deprecatedRuncConfig))
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedRuncConfig))
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(deprecatedRuncConfig))

			}),
		Entry("AKSUbuntu2404 containerd v2 CRI plugin config should not have deprecated cni features", "AKSUbuntu2404+NetworkPolicy", ">=1.32.x",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2404
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.32.0"
				// to have cni plugin non-default
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = NetworkPluginKubenet
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy = NetworkPolicyAntrea
			}, func(o *nodeBootstrappingOutput) {
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedCniV2Config := `
[plugins."io.containerd.cri.v1.runtime".cni]
  bin_dir = "/opt/cni/bin"
  conf_dir = "/etc/cni/net.d"
  conf_template = "/etc/containerd/kubenet_template.conf"
`
				deprecatedCniV1Config := `
  [plugins."io.containerd.grpc.v1.cri".cni]
    bin_dir = "/opt/cni/bin"
    conf_dir = "/etc/cni/net.d"
    conf_template = "/etc/containerd/kubenet_template.conf"
`
				Expect(expectedCniV2Config).NotTo(Equal(deprecatedCniV1Config))
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedCniV2Config))
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(deprecatedCniV1Config))
			}),

		Entry("AKSUbuntu2404 containerd v2 CRI plugin config should have version set to 3", "AKSUbuntu2404+Containerd2", ">=1.32.x",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2404
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.32.0"
			}, func(o *nodeBootstrappingOutput) {
				expectedVersion := `version = 3`

				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(containerdConfigFileContent).To(ContainSubstring(expectedVersion))

				containerdConfigNoGPUFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(containerdConfigNoGPUFileContent).To(ContainSubstring(expectedVersion))
			}),
	)
})
