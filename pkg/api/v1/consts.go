package v1


const (
	Ubuntu2204ImageFamily = "Ubuntu2204"
	AzureLinuxImageFamily = "AzureLinux"
	
	AKSUbuntuCommunityGallery     = "AKSUbuntu-38d80f77-467a-481f-a8d4-09b6d4220bd2"
	AKSAzureLinuxCommunityGallery = "AKSAzureLinux-f7c7cda5-1c9a-4bdc-a222-9614c968580b"
)

const (
	Kubenet = "kubenet"
	Azure = "azure"
	
	Calico = "calico" 
	Cilium = "cilium"

)

const (
	NetworkPolicyCalico = Calico 
	
	NetworkPolicyCilium = Cilium  
	NetworkPluginCilium = Cilium 

	NetworkPolicyAzure = Azure
	NetworkPluginAzure = Azure
	
	NetworkPluginKubenet = Kubenet 


	NetworkPluginModeOverlay = "overlay" 
	NetworkPluginModeNone = "none"
)

const (
	Nvidia470CudaDriverVersion = "cuda-470.82.01"
	Nvidia510CudaDriverVersion = "cuda-510.47.03"
	Nvidia525CudaDriverVersion = "cuda-525.85.12"
	Nvidia510GridDriverVersion = "grid-510.73.08"
	Nvidia535GridDriverVersion = "grid-535.54.03"
	
	// These SHAs will change once we update aks-gpu images in aks-gpu repository. We do that fairly rarely at this time
	AKSGPUGridSHA = "sha-20ffa2"
	AKSGPUCudaSHA = "sha-ff213d"
)
