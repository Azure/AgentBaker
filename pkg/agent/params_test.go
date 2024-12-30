package agent

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Assert Params required for cse_cmd.sh", func() {
	Describe("Test required params are set for linux", func() {
		var config *datamodel.NodeBootstrappingConfiguration
		BeforeEach(func() {
			config = &datamodel.NodeBootstrappingConfiguration{
				ContainerService: &datamodel.ContainerService{
					Properties: &datamodel.Properties{
						HostedMasterProfile: &datamodel.HostedMasterProfile{},
						OrchestratorProfile: &datamodel.OrchestratorProfile{
							KubernetesConfig: &datamodel.KubernetesConfig{
								ContainerRuntimeConfig: map[string]string{},
							},
						},
					},
				},
				AgentPoolProfile: &datamodel.AgentPoolProfile{
					KubernetesConfig: &datamodel.KubernetesConfig{},
				},
				CloudSpecConfig: &datamodel.AzureEnvironmentSpecConfig{},
				K8sComponents:   &datamodel.K8sComponents{},
			}
		})

		Describe("containerd2 required params are set", func() {
			It("the min supported k8s version is set with orchestrator profile indicates isKubernetes()", func() {
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = datamodel.Kubernetes
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.31"
				paramsMap := getParameters(config)
				Expect(GetParamKey(paramsMap, "containerd2MinKubeVersion")).To(Equal("1.32"))
				Expect(GetParamKey(paramsMap, "kubernetesVersion")).To(Equal(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion))
			})

			It("containerd version is set with agentpool profile, only if isKubernetes() and containerd", func() {
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = datamodel.Kubernetes
				config.AgentPoolProfile.KubernetesConfig.ContainerRuntime = "containerd"
				config.ContainerdVersion = "1.6.5"
				config.ContainerdPackageURL = "https://user-supplied-url.com/releases"
				paramsMap := getParameters(config)

				Expect(GetParamKey(paramsMap, "containerd2MinKubeVersion")).To(Equal("1.32"))
				Expect(GetParamKey(paramsMap, "containerRuntime")).To(Equal(config.AgentPoolProfile.KubernetesConfig.ContainerRuntime))
				Expect(GetParamKey(paramsMap, "containerdPackageURL")).To(Equal(config.ContainerdPackageURL))
			})

			It("containerd version is not set with when isKubernetes() is false", func() {
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorType = "otherType"
				paramsMap := getParameters(config)
				Expect(HasNotParamKey(paramsMap, "containerd2MinKubeVersion")).To(BeTrue())
				Expect(HasNotParamKey(paramsMap, "containerRuntime")).To(BeTrue())
				Expect(HasNotParamKey(paramsMap, "containerdPackageURL")).To(BeTrue())
			})
		})
	})
})

func HasNotParamKey(paramsMap paramsMap, key string) bool {
	_, ok := paramsMap[key]
	return !ok
}

func GetParamKey(inputParams paramsMap, key string) any {
	actual, _ := inputParams[key]
	return actual.(paramsMap)["value"]
}
