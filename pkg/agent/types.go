package agent

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// KeyVaultID represents a KeyVault instance on Azure
type KeyVaultID struct {
	ID string `json:"id"`
}

// KeyVaultRef represents a reference to KeyVault instance on Azure
type KeyVaultRef struct {
	KeyVault      KeyVaultID `json:"keyVault"`
	SecretName    string     `json:"secretName"`
	SecretVersion string     `json:"secretVersion,omitempty"`
}

// NodeBootstrappingConfiguration represents configurations for node bootstrapping
type NodeBootstrappingConfiguration struct {
	ContainerService              *datamodel.ContainerService
	CloudSpecConfig               *datamodel.AzureEnvironmentSpecConfig
	K8sComponents                 map[string]string
	AgentPoolProfile              *datamodel.AgentPoolProfile
	TenantID                      string
	SubscriptionID                string
	ResourceGroupName             string
	UserAssignedIdentityClientID  string
	ConfigGPUDriverIfNeeded       bool
	EnableGPUDevicePluginIfNeeded bool
	EnableDynamicKubelet          bool
}

// AKSKubeletConfiguration contains the configuration for the Kubelet that AKS set
// this is a subset of KubeletConfiguration defined in https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubelet/config/v1beta1/types.go
// changed metav1.Duration to Duration and pointers to values to simplify translation
type AKSKubeletConfiguration struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	// +optional
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
	// staticPodPath is the path to the directory containing local (static) pods to
	// run, or the path to a single static pod file.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// the set of static pods specified at the new path may be different than the
	// ones the Kubelet initially started with, and this may disrupt your node.
	// Default: ""
	// +optional
	StaticPodPath string `json:"staticPodPath,omitempty"`
	// address is the IP address for the Kubelet to serve on (set to 0.0.0.0
	// for all interfaces).
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Default: "0.0.0.0"
	// +optional
	Address string `json:"address,omitempty"`
	// readOnlyPort is the read-only port for the Kubelet to serve on with
	// no authentication/authorization.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Default: 0 (disabled)
	// +optional
	ReadOnlyPort int32 `json:"readOnlyPort,omitempty"`
	// tlsCertFile is the file containing x509 Certificate for HTTPS. (CA cert,
	// if any, concatenated after server cert). If tlsCertFile and
	// tlsPrivateKeyFile are not provided, a self-signed certificate
	// and key are generated for the public address and saved to the directory
	// passed to the Kubelet's --cert-dir flag.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Default: ""
	// +optional
	TLSCertFile string `json:"tlsCertFile,omitempty"`
	// tlsPrivateKeyFile is the file containing x509 private key matching tlsCertFile
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Default: ""
	// +optional
	TLSPrivateKeyFile string `json:"tlsPrivateKeyFile,omitempty"`
	// TLSCipherSuites is the list of allowed cipher suites for the server.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Default: nil
	// +optional
	TLSCipherSuites []string `json:"tlsCipherSuites,omitempty"`
	// rotateCertificates enables client certificate rotation. The Kubelet will request a
	// new certificate from the certificates.k8s.io API. This requires an approver to approve the
	// certificate signing requests.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// disabling it may disrupt the Kubelet's ability to authenticate with the API server
	// after the current certificate expires.
	// Default: false
	// +optional
	RotateCertificates bool `json:"rotateCertificates,omitempty"`
	// authentication specifies how requests to the Kubelet's server are authenticated
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Defaults:
	//   anonymous:
	//     enabled: false
	//   webhook:
	//     enabled: true
	//     cacheTTL: "2m"
	// +optional
	Authentication KubeletAuthentication `json:"authentication"`
	// authorization specifies how requests to the Kubelet's server are authorized
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may disrupt components that interact with the Kubelet server.
	// Defaults:
	//   mode: Webhook
	//   webhook:
	//     cacheAuthorizedTTL: "5m"
	//     cacheUnauthorizedTTL: "30s"
	// +optional
	Authorization KubeletAuthorization `json:"authorization"`
	// eventRecordQPS is the maximum event creations per second. If 0, there
	// is no limit enforced.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may impact scalability by changing the amount of traffic produced by
	// event creations.
	// Default: 5
	// +optional
	EventRecordQPS int32 `json:"eventRecordQPS,omitempty"`
	// clusterDomain is the DNS domain for this cluster. If set, kubelet will
	// configure all containers to search this domain in addition to the
	// host's search domains.
	// Dynamic Kubelet Config (beta): Dynamically updating this field is not recommended,
	// as it should be kept in sync with the rest of the cluster.
	// Default: ""
	// +optional
	ClusterDomain string `json:"clusterDomain,omitempty"`
	// clusterDNS is a list of IP addresses for the cluster DNS server. If set,
	// kubelet will configure all containers to use this for DNS resolution
	// instead of the host's DNS servers.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// changes will only take effect on Pods created after the update. Draining
	// the node is recommended before changing this field.
	// Default: nil
	// +optional
	ClusterDNS []string `json:"clusterDNS,omitempty"`
	// streamingConnectionIdleTimeout is the maximum time a streaming connection
	// can be idle before the connection is automatically closed.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may impact components that rely on infrequent updates over streaming
	// connections to the Kubelet server.
	// Default: "4h"
	// +optional
	StreamingConnectionIdleTimeout Duration `json:"streamingConnectionIdleTimeout,omitempty"`
	// nodeStatusUpdateFrequency is the frequency that kubelet computes node
	// status. If node lease feature is not enabled, it is also the frequency that
	// kubelet posts node status to master.
	// Note: When node lease feature is not enabled, be cautious when changing the
	// constant, it must work with nodeMonitorGracePeriod in nodecontroller.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may impact node scalability, and also that the node controller's
	// nodeMonitorGracePeriod must be set to N*NodeStatusUpdateFrequency,
	// where N is the number of retries before the node controller marks
	// the node unhealthy.
	// Default: "10s"
	// +optional
	NodeStatusUpdateFrequency Duration `json:"nodeStatusUpdateFrequency,omitempty"`
	// imageGCHighThresholdPercent is the percent of disk usage after which
	// image garbage collection is always run. The percent is calculated as
	// this field value out of 100.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may trigger or delay garbage collection, and may change the image overhead
	// on the node.
	// Default: 85
	// +optional
	ImageGCHighThresholdPercent int32 `json:"imageGCHighThresholdPercent,omitempty"`
	// imageGCLowThresholdPercent is the percent of disk usage before which
	// image garbage collection is never run. Lowest disk usage to garbage
	// collect to. The percent is calculated as this field value out of 100.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may trigger or delay garbage collection, and may change the image overhead
	// on the node.
	// Default: 80
	// +optional
	ImageGCLowThresholdPercent int32 `json:"imageGCLowThresholdPercent,omitempty"`
	// Enable QoS based Cgroup hierarchy: top level cgroups for QoS Classes
	// And all Burstable and BestEffort pods are brought up under their
	// specific top level QoS cgroup.
	// Dynamic Kubelet Config (beta): This field should not be updated without a full node
	// reboot. It is safest to keep this value the same as the local config.
	// Default: true
	// +optional
	CgroupsPerQOS bool `json:"cgroupsPerQOS,omitempty"`
	// maxPods is the number of pods that can run on this Kubelet.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// changes may cause Pods to fail admission on Kubelet restart, and may change
	// the value reported in Node.Status.Capacity[v1.ResourcePods], thus affecting
	// future scheduling decisions. Increasing this value may also decrease performance,
	// as more Pods can be packed into a single node.
	// Default: 110
	// +optional
	MaxPods int32 `json:"maxPods,omitempty"`
	// PodPidsLimit is the maximum number of pids in any pod.
	// Requires the SupportPodPidsLimit feature gate to be enabled.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// lowering it may prevent container processes from forking after the change.
	// Default: -1
	// +optional
	PodPidsLimit int64 `json:"podPidsLimit,omitempty"`
	// ResolverConfig is the resolver configuration file used as the basis
	// for the container DNS resolution configuration.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// changes will only take effect on Pods created after the update. Draining
	// the node is recommended before changing this field.
	// Default: "/etc/resolv.conf"
	// +optional
	ResolverConfig string `json:"resolvConf,omitempty"`
	// Map of signal names to quantities that defines hard eviction thresholds. For example: {"memory.available": "300Mi"}.
	// To explicitly disable, pass a 0% or 100% threshold on an arbitrary resource.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may trigger or delay Pod evictions.
	// Default:
	//   memory.available:  "100Mi"
	//   nodefs.available:  "10%"
	//   nodefs.inodesFree: "5%"
	//   imagefs.available: "15%"
	// +optional
	EvictionHard map[string]string `json:"evictionHard,omitempty"`
	// protectKernelDefaults, if true, causes the Kubelet to error if kernel
	// flags are not as it expects. Otherwise the Kubelet will attempt to modify
	// kernel flags to match its expectation.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// enabling it may cause the Kubelet to crash-loop if the Kernel is not configured as
	// Kubelet expects.
	// Default: false
	// +optional
	ProtectKernelDefaults bool `json:"protectKernelDefaults,omitempty"`
	// featureGates is a map of feature names to bools that enable or disable alpha/experimental
	// features. This field modifies piecemeal the built-in default values from
	// "k8s.io/kubernetes/pkg/features/kube_features.go".
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider the
	// documentation for the features you are enabling or disabling. While we
	// encourage feature developers to make it possible to dynamically enable
	// and disable features, some changes may require node reboots, and some
	// features may require careful coordination to retroactively disable.
	// Default: nil
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`

	/* the following fields are meant for Node Allocatable */

	// systemReserved is a set of ResourceName=ResourceQuantity (e.g. cpu=200m,memory=150G)
	// pairs that describe resources reserved for non-kubernetes components.
	// Currently only cpu and memory are supported.
	// See http://kubernetes.io/docs/user-guide/compute-resources for more detail.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may not be possible to increase the reserved resources, because this
	// requires resizing cgroups. Always look for a NodeAllocatableEnforced event
	// after updating this field to ensure that the update was successful.
	// Default: nil
	// +optional
	SystemReserved map[string]string `json:"systemReserved,omitempty"`
	// A set of ResourceName=ResourceQuantity (e.g. cpu=200m,memory=150G) pairs
	// that describe resources reserved for kubernetes system components.
	// Currently cpu, memory and local storage for root file system are supported.
	// See http://kubernetes.io/docs/user-guide/compute-resources for more detail.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// it may not be possible to increase the reserved resources, because this
	// requires resizing cgroups. Always look for a NodeAllocatableEnforced event
	// after updating this field to ensure that the update was successful.
	// Default: nil
	// +optional
	KubeReserved map[string]string `json:"kubeReserved,omitempty"`
	// This flag specifies the various Node Allocatable enforcements that Kubelet needs to perform.
	// This flag accepts a list of options. Acceptable options are `none`, `pods`, `system-reserved` & `kube-reserved`.
	// If `none` is specified, no other options may be specified.
	// Refer to [Node Allocatable](https://git.k8s.io/community/contributors/design-proposals/node/node-allocatable.md) doc for more information.
	// Dynamic Kubelet Config (beta): If dynamically updating this field, consider that
	// removing enforcements may reduce the stability of the node. Alternatively, adding
	// enforcements may reduce the stability of components which were using more than
	// the reserved amount of resources; for example, enforcing kube-reserved may cause
	// Kubelets to OOM if it uses more than the reserved resources, and enforcing system-reserved
	// may cause system daemons to OOM if they use more than the reserved resources.
	// Default: ["pods"]
	// +optional
	EnforceNodeAllocatable []string `json:"enforceNodeAllocatable,omitempty"`
}

type Duration string

// below are copied from Kubernetes
type KubeletAuthentication struct {
	// x509 contains settings related to x509 client certificate authentication
	// +optional
	X509 KubeletX509Authentication `json:"x509"`
	// webhook contains settings related to webhook bearer token authentication
	// +optional
	Webhook KubeletWebhookAuthentication `json:"webhook"`
	// anonymous contains settings related to anonymous authentication
	// +optional
	Anonymous KubeletAnonymousAuthentication `json:"anonymous"`
}

type KubeletX509Authentication struct {
	// clientCAFile is the path to a PEM-encoded certificate bundle. If set, any request presenting a client certificate
	// signed by one of the authorities in the bundle is authenticated with a username corresponding to the CommonName,
	// and groups corresponding to the Organization in the client certificate.
	// +optional
	ClientCAFile string `json:"clientCAFile,omitempty"`
}

type KubeletWebhookAuthentication struct {
	// enabled allows bearer token authentication backed by the tokenreviews.authentication.k8s.io API
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// cacheTTL enables caching of authentication results
	// +optional
	CacheTTL Duration `json:"cacheTTL,omitempty"`
}

type KubeletAnonymousAuthentication struct {
	// enabled allows anonymous requests to the kubelet server.
	// Requests that are not rejected by another authentication method are treated as anonymous requests.
	// Anonymous requests have a username of system:anonymous, and a group name of system:unauthenticated.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

type KubeletAuthorization struct {
	// mode is the authorization mode to apply to requests to the kubelet server.
	// Valid values are AlwaysAllow and Webhook.
	// Webhook mode uses the SubjectAccessReview API to determine authorization.
	// +optional
	Mode KubeletAuthorizationMode `json:"mode,omitempty"`

	// webhook contains settings related to Webhook authorization.
	// +optional
	Webhook KubeletWebhookAuthorization `json:"webhook"`
}

type KubeletAuthorizationMode string

type KubeletWebhookAuthorization struct {
	// cacheAuthorizedTTL is the duration to cache 'authorized' responses from the webhook authorizer.
	// +optional
	CacheAuthorizedTTL Duration `json:"cacheAuthorizedTTL,omitempty"`
	// cacheUnauthorizedTTL is the duration to cache 'unauthorized' responses from the webhook authorizer.
	// +optional
	CacheUnauthorizedTTL Duration `json:"cacheUnauthorizedTTL,omitempty"`
}
