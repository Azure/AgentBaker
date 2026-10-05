package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

const expectedlocalDNSCorefileWithoutOverrides = `# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    reload
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
`

var _ = Describe("LocalDNS template methods", func() {
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
			AgentPoolProfile: &datamodel.AgentPoolProfile{},
		}
	})

	Describe(".ShouldEnableLocalDNS()", func() {
		// Expect ShouldEnableLocalDNS func to return false if LocalDNSProfile is nil.
		It("returns false when AgentPoolProfile is nil", func() {
			config.AgentPoolProfile = nil
			Expect(config.AgentPoolProfile.ShouldEnableLocalDNS()).To(BeFalse())
		})
		// Expect ShouldEnableLocalDNS func to return false if LocalDNSProfile is nil.
		It("returns false when LocalDNSProfile is nil", func() {
			config.AgentPoolProfile.LocalDNSProfile = nil
			Expect(config.AgentPoolProfile.ShouldEnableLocalDNS()).To(BeFalse())
		})
		// Expect ShouldEnableLocalDNS func to return false if LocalDNSProfile is empty.
		It("returns false when LocalDNSProfile is empty", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{}
			Expect(config.AgentPoolProfile.ShouldEnableLocalDNS()).To(BeFalse())
		})
		// Expect ShouldEnableLocalDNS func to return false if EnableLocalDNS is false.
		It("returns false when EnableLocalDNS is false", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: false,
			}
			Expect(config.AgentPoolProfile.ShouldEnableLocalDNS()).To(BeFalse())
		})
		// Expect ShouldEnableLocalDNS func to return true if EnableLocalDNS is true.
		It("returns true when EnableLocalDNS is true", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
			}
			Expect(config.AgentPoolProfile.ShouldEnableLocalDNS()).To(BeTrue())
		})
	})

	Describe(".GetLocalDNSCPULimitInPercentage()", func() {
		// Expect default CPUlimit to be returned.
		It("returns default CPULimit - 200.0%", func() {
			config.AgentPoolProfile.LocalDNSProfile = nil
			Expect(config.AgentPoolProfile.GetLocalDNSCPULimitInPercentage()).To(ContainSubstring("200.0%"))
		})
		// Expect default CPUlimit to be returned if CPULimitInMilliCores is nil.
		It("returns default CPULimit - 200.0% when CPULimitInMilliCores is nil", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: nil,
			}
			Expect(config.AgentPoolProfile.GetLocalDNSCPULimitInPercentage()).To(ContainSubstring("200.0%"))
		})
		// Expect input value to be returned even if EnableLocalDNS is false.
		It("returns input value of CPULimit - 500.0% even when EnableLocalDNS is false", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       false,
				CPULimitInMilliCores: to.Int32Ptr(5000),
			}
			Expect(config.AgentPoolProfile.GetLocalDNSCPULimitInPercentage()).To(ContainSubstring("500.0%"))
		})
		// Expect input value to be returned if EnableLocalDNS is true.
		It("returns input value of CPULimit - 489.7%", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: to.Int32Ptr(4897),
			}
			Expect(config.AgentPoolProfile.GetLocalDNSCPULimitInPercentage()).To(ContainSubstring("489.7%"))
		})
	})

	Describe(".GetLocalDNSMemoryLimitInMB()", func() {
		// Expect default memorylimit to be returned if LocalDNSProfile is nil.
		It("returns default MemoryLimitInMB - 128M", func() {
			config.AgentPoolProfile.LocalDNSProfile = nil
			Expect(config.AgentPoolProfile.GetLocalDNSMemoryLimitInMB()).To(ContainSubstring("128M"))
		})
		// Expect default memorylimit to be returned if MemoryLimitInMB is nil.
		It("returns default MemoryLimitInMB - 128M", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:  true,
				MemoryLimitInMB: nil,
			}
			Expect(config.AgentPoolProfile.GetLocalDNSMemoryLimitInMB()).To(ContainSubstring("128M"))
		})
		// Expect input value of memorylimit to be returned if EnableLocalDNS is false.
		It("returns input value of MemoryLimitInMB - 438M", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:  false,
				MemoryLimitInMB: to.Int32Ptr(438),
			}
			Expect(config.AgentPoolProfile.GetLocalDNSMemoryLimitInMB()).To(ContainSubstring("438M"))
		})
		// Expect input value of memorylimit to be returned if EnableLocalDNS is true.
		It("returns input value of MemoryLimitInMB - 1024M", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:  true,
				MemoryLimitInMB: to.Int32Ptr(1024),
			}
			Expect(config.AgentPoolProfile.GetLocalDNSMemoryLimitInMB()).To(ContainSubstring("1024M"))
		})
	})

	Describe("GetLocalDNSCriticalFQDNs template func", func() {
		It("returns empty string when LocalDNSProfile is nil", func() {
			config.AgentPoolProfile.LocalDNSProfile = nil
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSCriticalFQDNs"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal(""))
		})
		It("returns empty string when CriticalFQDNs is nil", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
				CriticalFQDNs:  nil,
			}
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSCriticalFQDNs"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal(""))
		})
		It("returns comma-separated FQDNs", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
				CriticalFQDNs: []string{
					"mcr.microsoft.com",
					"packages.microsoft.com",
					"login.microsoftonline.com",
				},
			}
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSCriticalFQDNs"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal("mcr.microsoft.com,packages.microsoft.com,login.microsoftonline.com"))
		})
		It("returns single FQDN without trailing comma", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
				CriticalFQDNs:  []string{"mcr.microsoft.com"},
			}
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSCriticalFQDNs"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal("mcr.microsoft.com"))
		})
	})

	Describe("GetLocalDNSHostsPluginRefreshIntervalInSeconds template func", func() {
		It("returns empty string when LocalDNSProfile is nil", func() {
			config.AgentPoolProfile.LocalDNSProfile = nil
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSHostsPluginRefreshIntervalInSeconds"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal(""))
		})
		It("returns empty string when refresh interval is nil", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
			}
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSHostsPluginRefreshIntervalInSeconds"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal(""))
		})
		It("returns empty string when refresh interval is non-positive", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:                      true,
				HostsPluginRefreshIntervalInSeconds: to.Int32Ptr(0),
			}
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSHostsPluginRefreshIntervalInSeconds"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal(""))
		})
		It("returns the refresh interval in seconds", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:                      true,
				HostsPluginRefreshIntervalInSeconds: to.Int32Ptr(30),
			}
			funcMap := getContainerServiceFuncMap(config)
			fn, ok := funcMap["GetLocalDNSHostsPluginRefreshIntervalInSeconds"].(func() string)
			Expect(ok).To(BeTrue())
			Expect(fn()).To(Equal("30"))
		})
	})

	Describe(".GetGeneratedLocalDNSCoreFile()", func() {
		// Expect no error and a non-empty corefile when LocalDNSOverrides are nil.
		It("handles nil LocalDNSOverrides", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: to.Int32Ptr(2008),
				MemoryLimitInMB:      to.Int32Ptr(128),
				VnetDNSOverrides:     nil,
				KubeDNSOverrides:     nil,
			}
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, true)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(BeEmpty())
			Expect(localDNSCoreFile).To(ContainSubstring(expectedlocalDNSCorefileWithoutOverrides))
		})

		// Expect no error and a non-empty corefile when LocalDNSOverrides are empty.
		It("handles empty LocalDNSOverrides", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: to.Int32Ptr(2008),
				MemoryLimitInMB:      to.Int32Ptr(128),
				VnetDNSOverrides:     map[string]*datamodel.LocalDNSOverrides{},
				KubeDNSOverrides:     map[string]*datamodel.LocalDNSOverrides{},
			}
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, true)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(BeEmpty())
			Expect(localDNSCoreFile).To(ContainSubstring(expectedlocalDNSCorefileWithoutOverrides))
		})

		// Expect no error and a non-empty corefile when LocalDNSOverrides are empty.
		It("handles empty KubeDNSOverrides and non-empty VnetDNSOverrides", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: to.Int32Ptr(2008),
				MemoryLimitInMB:      to.Int32Ptr(128),
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Immediate",
					},
					"cluster.local": {
						QueryLogging:                "Error",
						Protocol:                    "ForceTCP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Disable",
					},
					"testdomain456.com": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Verify",
					},
				},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(2000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(72000),
						ServeStale:                  "Verify",
					},
				},
			}
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, true)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(BeEmpty())

			expectedlocalDNSCorefile := `# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    reload
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    log
    bind 169.254.10.10
    # Check /etc/localdns/hosts first for critical AKS FQDNs (mcr.microsoft.com, packages.aks.azure.com, etc.)
    hosts /etc/localdns/hosts {
        ttl 5
        reload 5s
        fallthrough
    }
    forward . 168.63.129.16 {
        prefer_udp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loadbalance
    loop
    nsid localdns
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    errors
    bind 169.254.10.10
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        servfail 0
    }
    loadbalance
    loop
    nsid localdns
    prometheus :9253
}
testdomain456.com:53 {
    log
    bind 169.254.10.10
    forward . 10.0.0.10 {
        prefer_udp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loadbalance
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    # Check /etc/localdns/hosts first for critical AKS FQDNs (mcr.microsoft.com, packages.aks.azure.com, etc.)
    hosts /etc/localdns/hosts {
        ttl 5
        reload 5s
        fallthrough
    }
    forward . 10.0.0.10 {
        prefer_udp
        policy sequential
        max_concurrent 2000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 72000s verify
        servfail 0
    }
    loadbalance
    loop
    nsid localdns-pod
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
`
			Expect(localDNSCoreFile).To(ContainSubstring(expectedlocalDNSCorefile))
		})

		// Expect no error and correct localdns corefile.
		It("generates a valid localdnsCorefile", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				CPULimitInMilliCores: to.Int32Ptr(2008),
				MemoryLimitInMB:      to.Int32Ptr(128),
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Verify",
					},
					"cluster.local": {
						QueryLogging:                "Error",
						Protocol:                    "ForceTCP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Disable",
					},
					"testdomain456.com": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Verify",
					},
				},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Verify",
					},
					"cluster.local": {
						QueryLogging:                "Log",
						Protocol:                    "ForceTCP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "RoundRobin",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Disable",
					},
					"testdomain567.com": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Random",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Immediate",
					},
				},
			}
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, true)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(BeEmpty())

			expectedlocalDNSCorefile := `# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    reload
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    log
    bind 169.254.10.10
    # Check /etc/localdns/hosts first for critical AKS FQDNs (mcr.microsoft.com, packages.aks.azure.com, etc.)
    hosts /etc/localdns/hosts {
        ttl 5
        reload 5s
        fallthrough
    }
    forward . 168.63.129.16 {
        prefer_udp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loadbalance
    loop
    nsid localdns
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    errors
    bind 169.254.10.10
    forward . 10.0.0.10 {
        force_tcp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        servfail 0
    }
    loadbalance
    loop
    nsid localdns
    prometheus :9253
}
testdomain456.com:53 {
    log
    bind 169.254.10.10
    forward . 10.0.0.10 {
        prefer_udp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.10:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loadbalance
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    # Check /etc/localdns/hosts first for critical AKS FQDNs (mcr.microsoft.com, packages.aks.azure.com, etc.)
    hosts /etc/localdns/hosts {
        ttl 5
        reload 5s
        fallthrough
    }
    forward . 10.0.0.10 {
        prefer_udp
        policy sequential
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s verify
        servfail 0
    }
    loadbalance
    loop
    nsid localdns-pod
    prometheus :9253
    template ANY ANY internal.cloudapp.net {
        match "^(?:[^.]+\.){4,}internal\.cloudapp\.net\.$"
        rcode NXDOMAIN
        fallthrough
    }
    template ANY ANY reddog.microsoft.com {
        rcode NXDOMAIN
    }
}
cluster.local:53 {
    log
    bind 169.254.10.11
    forward . 10.0.0.10 {
        force_tcp
        policy round_robin
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        servfail 0
    }
    loadbalance
    loop
    nsid localdns-pod
    prometheus :9253
}
testdomain567.com:53 {
    errors
    bind 169.254.10.11
    forward . 168.63.129.16 {
        prefer_udp
        policy random
        max_concurrent 1000
    }
    ready 169.254.10.11:8181
    cache 3600 {
        success 9984
        denial 9984
        serve_stale 3600s immediate
        servfail 0
    }
    loadbalance
    loop
    nsid localdns-pod
    prometheus :9253
}
`
			Expect(localDNSCoreFile).To(ContainSubstring(expectedlocalDNSCorefile))
		})

		It("omits failfast when explicitly disabled for localdns forward knobs", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                  "Log",
						Protocol:                      "PreferUDP",
						ForwardDestination:            "VnetDNS",
						ForwardPolicy:                 "Sequential",
						MaxConcurrent:                 to.Int32Ptr(1000),
						CacheDurationInSeconds:        to.Int32Ptr(3600),
						ServeStaleDurationInSeconds:   to.Int32Ptr(3600),
						ServeStale:                    "Immediate",
						FailfastAllUnhealthyUpstreams: to.BoolPtr(false),
						HealthCheck: &datamodel.LocalDNSHealthCheck{
							Duration: to.StringPtr("1s"),
						},
					},
				},
			}

			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).To(ContainSubstring("health_check 1s"))
			Expect(localDNSCoreFile).ToNot(ContainSubstring("failfast_all_unhealthy_upstreams"))
		})

		It("renders localdns forward health knobs", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": {
					QueryLogging: "Log", Protocol: "PreferUDP", ForwardDestination: "VnetDNS", ForwardPolicy: "Sequential",
					MaxConcurrent: to.Int32Ptr(1000), CacheDurationInSeconds: to.Int32Ptr(3600), ServeStaleDurationInSeconds: to.Int32Ptr(3600), ServeStale: "Immediate",
					FailfastAllUnhealthyUpstreams: to.BoolPtr(true),
					HealthCheck: &datamodel.LocalDNSHealthCheck{
						Duration: to.StringPtr("1s"),
						NoRec:    to.BoolPtr(true),
						Domain:   to.StringPtr("health.local."),
					},
				}},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": {
					QueryLogging: "Error", Protocol: "PreferUDP", ForwardDestination: "ClusterCoreDNS", ForwardPolicy: "Sequential",
					MaxConcurrent: to.Int32Ptr(1000), CacheDurationInSeconds: to.Int32Ptr(3600), ServeStaleDurationInSeconds: to.Int32Ptr(3600), ServeStale: "Immediate",
					HealthCheck: &datamodel.LocalDNSHealthCheck{Duration: to.StringPtr("2s"), Domain: to.StringPtr("")},
				}},
			}
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).To(ContainSubstring("health_check 1s no_rec domain health.local."))
			Expect(localDNSCoreFile).To(ContainSubstring("failfast_all_unhealthy_upstreams"))
			Expect(localDNSCoreFile).To(ContainSubstring("health_check 2s"))
			Expect(localDNSCoreFile).ToNot(ContainSubstring("domain \n"))
		})

		// serve_stale_policy requires CoreDNS >= 1.14.7 and is only valid alongside an
		// emitted serve_stale line, so the template must never render it on its own.
		It("renders serve_stale_policy only alongside serve_stale", func() {
			newOverride := func(serveStale, policy string) *datamodel.LocalDNSOverrides {
				return &datamodel.LocalDNSOverrides{
					QueryLogging: "Log", Protocol: "PreferUDP", ForwardDestination: "VnetDNS", ForwardPolicy: "Sequential",
					MaxConcurrent: to.Int32Ptr(1000), CacheDurationInSeconds: to.Int32Ptr(3600),
					ServeStaleDurationInSeconds: to.Int32Ptr(3600),
					ServeStale:                  serveStale,
					ServeStalePolicy:            policy,
				}
			}

			By("rendering the directive under the serve_stale line when the policy is set")
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:   true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": newOverride("Immediate", "PreferPositive")},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": newOverride("Verify", "PreferPositive")},
			}
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).To(ContainSubstring("serve_stale 3600s immediate\n        serve_stale_policy prefer_positive"))
			Expect(localDNSCoreFile).To(ContainSubstring("serve_stale 3600s verify\n        serve_stale_policy prefer_positive"))

			By("omitting the directive when no policy is set")
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:   true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": newOverride("Immediate", "")},
			}
			localDNSCoreFile, err = GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).To(ContainSubstring("serve_stale 3600s immediate"))
			Expect(localDNSCoreFile).ToNot(ContainSubstring("serve_stale_policy"))

			By("omitting the directive when serve_stale itself is disabled")
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:   true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": newOverride("Disable", "PreferPositive")},
			}
			localDNSCoreFile, err = GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(ContainSubstring("serve_stale"))

			By("omitting the directive when serve_stale carries an unrecognized value")
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:   true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{".": newOverride("Bogus", "PreferPositive")},
			}
			localDNSCoreFile, err = GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(ContainSubstring("serve_stale"))

			By("omitting the directive outside the default (\".\") server block")
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS: true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					"cluster.local":  newOverride("Immediate", "PreferPositive"),
					"testdomain.com": newOverride("Immediate", "PreferPositive"),
				},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{"cluster.local": newOverride("Verify", "PreferPositive")},
			}
			localDNSCoreFile, err = GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			// The per-domain blocks still get serve_stale; only the policy is withheld.
			Expect(localDNSCoreFile).To(ContainSubstring("serve_stale 3600s immediate"))
			Expect(localDNSCoreFile).To(ContainSubstring("serve_stale 3600s verify"))
			Expect(localDNSCoreFile).ToNot(ContainSubstring("serve_stale_policy"))
		})

		// Expect a valid corefile WITHOUT hosts plugin blocks when includeHostsPlugin=false.
		// This is the fallback corefile used when enableAKSLocalDNSHostsSetup fails at provisioning time.
		It("generates a valid localdnsCorefile without hosts plugin when includeHostsPlugin is false", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				EnableHostsPlugin:    true,
				CPULimitInMilliCores: to.Int32Ptr(2008),
				MemoryLimitInMB:      to.Int32Ptr(128),
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Immediate",
					},
				},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(2000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(72000),
						ServeStale:                  "Verify",
					},
				},
			}
			// Generate with includeHostsPlugin=false (the no-hosts fallback)
			localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())
			Expect(localDNSCoreFile).ToNot(BeEmpty())

			// The no-hosts corefile must NOT contain hosts plugin blocks
			Expect(localDNSCoreFile).ToNot(ContainSubstring("hosts /etc/localdns/hosts"))
			Expect(localDNSCoreFile).ToNot(ContainSubstring("# Check /etc/localdns/hosts"))

			// But it should still contain the standard corefile structure
			Expect(localDNSCoreFile).To(ContainSubstring("health-check.localdns.local:53"))
			Expect(localDNSCoreFile).To(ContainSubstring("bind 169.254.10.10"))
			Expect(localDNSCoreFile).To(ContainSubstring("bind 169.254.10.11"))
			Expect(localDNSCoreFile).To(ContainSubstring("forward . 168.63.129.16"))
			Expect(localDNSCoreFile).To(ContainSubstring("prefer_udp"))
			Expect(localDNSCoreFile).To(ContainSubstring("nsid localdns"))
			Expect(localDNSCoreFile).To(ContainSubstring("nsid localdns-pod"))
		})

		// Verify that includeHostsPlugin=true produces hosts blocks and includeHostsPlugin=false does not,
		// when using the same LocalDNSProfile configuration.
		It("produces different output for includeHostsPlugin true vs false", func() {
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:       true,
				EnableHostsPlugin:    true,
				CPULimitInMilliCores: to.Int32Ptr(2008),
				MemoryLimitInMB:      to.Int32Ptr(128),
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Log",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "VnetDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Immediate",
					},
				},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
					".": {
						QueryLogging:                "Error",
						Protocol:                    "PreferUDP",
						ForwardDestination:          "ClusterCoreDNS",
						ForwardPolicy:               "Sequential",
						MaxConcurrent:               to.Int32Ptr(1000),
						CacheDurationInSeconds:      to.Int32Ptr(3600),
						ServeStaleDurationInSeconds: to.Int32Ptr(3600),
						ServeStale:                  "Verify",
					},
				},
			}
			withHosts, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, true)
			Expect(err).To(BeNil())
			withoutHosts, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, false)
			Expect(err).To(BeNil())

			// With hosts should have the hosts plugin block
			Expect(withHosts).To(ContainSubstring("hosts /etc/localdns/hosts"))
			// Without hosts should NOT have it
			Expect(withoutHosts).ToNot(ContainSubstring("hosts /etc/localdns/hosts"))
			// Both should still be valid corefiles
			Expect(withHosts).To(ContainSubstring("health-check.localdns.local:53"))
			Expect(withoutHosts).To(ContainSubstring("health-check.localdns.local:53"))
		})
	})
})

func TestGenerateLocalDNSCoreFileLoadBalance(t *testing.T) {
	for _, includeHosts := range []bool{false, true} {
		t.Run(fmt.Sprintf("hosts=%t", includeHosts), func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:   true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{},
			}
			for _, overrides := range []map[string]*datamodel.LocalDNSOverrides{
				config.AgentPoolProfile.LocalDNSProfile.VnetDNSOverrides,
				config.AgentPoolProfile.LocalDNSProfile.KubeDNSOverrides,
			} {
				for _, zone := range []string{".", "cluster.local", "example.test"} {
					overrides[zone] = &datamodel.LocalDNSOverrides{
						ForwardDestination: "ClusterCoreDNS", ForwardPolicy: "Sequential",
						MaxConcurrent: to.Int32Ptr(1000), CacheDurationInSeconds: to.Int32Ptr(300),
						ServeStale: "Disable",
					}
				}
			}
			corefile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, includeHosts)
			require.NoError(t, err)
			// Check each server block, including custom zones, rather than only the root block.
			for _, block := range strings.Split(corefile, "\n}\n") {
				if strings.Contains(block, "\n    forward . ") {
					require.Equal(t, 1, strings.Count(block, "\n    loadbalance\n"), block)
					require.Contains(t, block, "policy sequential")
				} else {
					require.NotContains(t, block, "loadbalance", "health-check block must stay unchanged")
				}
			}
			require.Equal(t, 6, strings.Count(corefile, "\n    loadbalance\n"))
		})
	}
}
