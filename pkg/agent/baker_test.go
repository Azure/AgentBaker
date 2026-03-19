package agent

import (
	"archive/tar"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/barkimedes/go-deepcopy"
	base0_5 "github.com/coreos/butane/base/v0_5"
	flatcar1_1 "github.com/coreos/butane/config/flatcar/v1_1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vincent-petithory/dataurl"
)


// this regex looks for groups of the following forms, returning KEY and VALUE as submatches.
/* - KEY=VALUE
- KEY="VALUE"
- KEY=
- KEY="VALUE WITH WHITSPACE". */
const cseRegexString = `([^=\s]+)=(\"[^\"]*\"|[^\s]*)`

const expectedlocalDNSCorefileWithoutOverrides = `
# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
`


type decodedValue struct {
	encoding string
	value    string
	mode     int64
}
var _ = Describe("Assert generated customData and cseCmd", func() {
	Describe("Tests of template methods", func() {
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

		Describe(".HasDataDir()", func() {
			It("given there is no profile, it returns false", func() {
				Expect(HasDataDir(config)).To(BeFalse())
			})
			It("given there is a data dir, it returns true", func() {
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig["dataDir"] = "data dir"
				Expect(HasDataDir(config)).To(BeTrue())
			})
			It("given there is a temp disk, it returns true", func() {
				// test the actual string because this data is posted to agentbaker and we want to check a particular posted string
				// - rather than the value of our internal const is mariner.
				config.AgentPoolProfile.KubeletDiskType = "Temporary"
				Expect(HasDataDir(config)).To(BeTrue())
			})
		})

		Describe(".GetDataDir()", func() {
			It("given there is no profile, it returns an empty string", func() {
				Expect(GetDataDir(config)).To(BeEmpty())
			})
			It("given there is a data dir, it returns true", func() {
				config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ContainerRuntimeConfig["dataDir"] = "data dir"
				Expect(GetDataDir(config)).To(Equal("data dir"))
			})
			It("given there is a temp disk, it returns true", func() {
				// test the actual string because this data is posted to agentbaker and we want to check a particular posted string
				// - rather than the value of our internal const is mariner.
				config.AgentPoolProfile.KubeletDiskType = "Temporary"
				Expect(GetDataDir(config)).To(Equal("/mnt/aks/containers"))
			})
		})

		Describe(".GetKubernetesEndpoint()", func() {
			It("given there is no profile, it returns an empty string", func() {
				Expect(GetKubernetesEndpoint(config.ContainerService)).To(BeEmpty())
			})
			It("given there is an ip address, it returns the ip address", func() {
				config.ContainerService.Properties.HostedMasterProfile.IPAddress = "127.0.0.1"
				Expect(GetKubernetesEndpoint(config.ContainerService)).To(Equal("127.0.0.1"))
			})
			It("given there is n fqdn, it returns the fqdn", func() {
				config.ContainerService.Properties.HostedMasterProfile.FQDN = "fqdn"
				Expect(GetKubernetesEndpoint(config.ContainerService)).To(Equal("fqdn"))
			})
			It("given there is an ip address and a fqdn, it returns the ip address", func() {
				config.ContainerService.Properties.HostedMasterProfile.IPAddress = "127.0.0.1"
				config.ContainerService.Properties.HostedMasterProfile.FQDN = "fqdn"
				Expect(GetKubernetesEndpoint(config.ContainerService)).To(Equal("127.0.0.1"))
			})
		})

		Describe(".getPortRangeEndValue()", func() {
			It("given a port range with 2 numbers, it returns an the second number", func() {
				Expect(getPortRangeEndValue("1 2")).To(Equal(2))
			})
			It("given a port range with 3 numbers, it returns an the second number", func() {
				Expect(getPortRangeEndValue("1 2 3")).To(Equal(2))
			})
		})

		Describe(".areCustomCATrustCertsPopulated()", func() {
			It("given an empty profile, it returns false", func() {
				Expect(areCustomCATrustCertsPopulated(*config)).To(BeFalse())
			})
			It("given no list of certs, it returns false", func() {
				config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{}
				Expect(areCustomCATrustCertsPopulated(*config)).To(BeFalse())
			})
			It("given an empty list of certs, it returns false", func() {
				config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{},
				}
				Expect(areCustomCATrustCertsPopulated(*config)).To(BeFalse())
			})
			It("given a single custom ca cert, it returns true", func() {
				config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{"mock cert value"},
				}
				Expect(areCustomCATrustCertsPopulated(*config)).To(BeTrue())
			})
			It("given 4 custom ca certs, it returns true", func() {
				config.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{"cert1", "cert2", "cert3", "cert4"},
				}
				Expect(areCustomCATrustCertsPopulated(*config)).To(BeTrue())
			})
		})

		Describe(".isMariner()", func() {
			It("given an empty string, that is not mariner", func() {
				Expect(isMariner("")).To(BeFalse())
			})
			It("given datamodel.OSSKUCBLMariner, that is mariner", func() {
				// test the actual string because this data is posted to agentbaker and we want to check a particular posted string
				// is mariner - rather than the value of our internal const is mariner.
				Expect(isMariner("CBLMariner")).To(BeTrue())
			})
			It("given datamodel.OSSKUMariner, that is mariner", func() {
				// test the actual string because this data is posted to agentbaker and we want to check a particular posted string
				// is mariner - rather than the value of our internal const is mariner.
				Expect(isMariner("Mariner")).To(BeTrue())
			})
			It("given datamodel.OSSKUAzureLinux, that is mariner", func() {
				// test the actual string because this data is posted to agentbaker and we want to check a particular posted string
				// is mariner - rather than the value of our internal const is mariner.
				Expect(isMariner("AzureLinux")).To(BeTrue())
			})
			It("given ubuntu, that is not mariner", func() {
				Expect(isMariner("Ubuntu")).To(BeFalse())
			})
		})

		// ------------------------------- Start of tests related to Localdns ---------------------------------------
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

		Describe(".GetGeneratedLocalDNSCoreFile()", func() {
			// Expect an error from GenerateLocalDNSCoreFile if template is invalid.
			It("returns an error when template parsing fails", func() {
				config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
					EnableLocalDNS:       true,
					CPULimitInMilliCores: to.Int32Ptr(2008),
					MemoryLimitInMB:      to.Int32Ptr(128),
					VnetDNSOverrides:     nil,
					KubeDNSOverrides:     nil,
				}
				invalidTemplate := "{{.InvalidField}}"
				_, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, invalidTemplate)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to execute localdns corefile template"))
			})

			// Expect no error and a non-empty corefile when LocalDNSOverrides are nil.
			It("handles nil LocalDNSOverrides", func() {
				config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
					EnableLocalDNS:       true,
					CPULimitInMilliCores: to.Int32Ptr(2008),
					MemoryLimitInMB:      to.Int32Ptr(128),
					VnetDNSOverrides:     nil,
					KubeDNSOverrides:     nil,
				}
				localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
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
				localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
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
				localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).To(BeNil())
				Expect(localDNSCoreFile).ToNot(BeEmpty())

				expectedlocalDNSCorefile := `
# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    log
    bind 169.254.10.10
    forward . 168.63.129.16 {
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
    loop
    nsid localdns
    prometheus :9253
}
testdomain456.com:53 {
    log
    bind 169.254.10.10
    forward . 10.0.0.10 {
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
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
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
				localDNSCoreFile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).To(BeNil())
				Expect(localDNSCoreFile).ToNot(BeEmpty())

				expectedlocalDNSCorefile := `
# ***********************************************************************************
# WARNING: Changes to this file will be overwritten and not persisted.
# ***********************************************************************************
# whoami (used for health check of DNS)
health-check.localdns.local:53 {
    bind 169.254.10.10 169.254.10.11
    whoami
}
# VnetDNS overrides apply to DNS traffic from pods with dnsPolicy:default or kubelet (referred to as VnetDNS traffic).
.:53 {
    log
    bind 169.254.10.10
    forward . 168.63.129.16 {
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
    loop
    nsid localdns
    prometheus :9253
}
testdomain456.com:53 {
    log
    bind 169.254.10.10
    forward . 10.0.0.10 {
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
    loop
    nsid localdns
    prometheus :9253
}
# KubeDNS overrides apply to DNS traffic from pods with dnsPolicy:ClusterFirst (referred to as KubeDNS traffic).
.:53 {
    errors
    bind 169.254.10.11
    forward . 10.0.0.10 {
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
    loop
    nsid localdns-pod
    prometheus :9253
}
testdomain567.com:53 {
    errors
    bind 169.254.10.11
    forward . 168.63.129.16 {
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
    loop
    nsid localdns-pod
    prometheus :9253
}
`
				Expect(localDNSCoreFile).To(ContainSubstring(expectedlocalDNSCorefile))
			})
		})
	})
})

type tarEntry struct {
	path string
	*decodedValue
}

func decodeTarFiles(data []byte) ([]tarEntry, error) {
	files := make([]tarEntry, 0)
	reader := tar.NewReader(bytes.NewReader(data))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		contents, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		path := "/" + strings.TrimPrefix(header.Name, "/")
		files = append(files, tarEntry{
			path: path,
			decodedValue: &decodedValue{
				value: string(contents),
				mode:  header.Mode,
			},
		})
	}
	return files, nil
}

func getDecodedVarsFromCseCmd(data []byte) (map[string]string, error) {
	cseRegex := regexp.MustCompile(cseRegexString)
	cseVariableList := cseRegex.FindAllStringSubmatch(string(data), -1)
	vars := make(map[string]string)

	for _, cseVar := range cseVariableList {
		if len(cseVar) < 3 {
			return nil, fmt.Errorf("expected 3 results (match, key, value) from regex, found %d, result %q", len(cseVar), cseVar)
		}

		key := cseVar[1]
		val := getValueWithoutQuotes(cseVar[2])

		vars[key] = val
	}

	return vars, nil
}

func getValueWithoutQuotes(value string) string {
	if len(value) > 1 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}

var _ = Describe("Test normalizeResourceGroupNameForLabel", func() {
	It("should return the correct normalized resource group name", func() {
		Expect(normalizeResourceGroupNameForLabel("hello")).To(Equal("hello"))
		Expect(normalizeResourceGroupNameForLabel("hel(lo")).To(Equal("hel-lo"))
		Expect(normalizeResourceGroupNameForLabel("hel)lo")).To(Equal("hel-lo"))
		var s string
		for i := 0; i < 63; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s)).To(Equal(s))
		Expect(normalizeResourceGroupNameForLabel(s + "1")).To(Equal(s))

		s = ""
		for i := 0; i < 62; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s + "(")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + ")")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + "-")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + "_")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel(s + ".")).To(Equal(s + "z"))
		Expect(normalizeResourceGroupNameForLabel("")).To(Equal(""))
		Expect(normalizeResourceGroupNameForLabel("z")).To(Equal("z"))

		// Add z, not replacing ending - with z, if name is short
		Expect(normalizeResourceGroupNameForLabel("-")).To(Equal("-z"))

		s = ""
		for i := 0; i < 61; i++ {
			s += "0"
		}
		Expect(normalizeResourceGroupNameForLabel(s + "-")).To(Equal(s + "-z"))
	})
})

var _ = Describe("GetGPUDriverVersion", func() {
	It("should use 470 with nc v1", func() {
		Expect(GetGPUDriverVersion("standard_nc6")).To(Equal(datamodel.Nvidia470CudaDriverVersion))
	})
	It("should use cuda with nc v3", func() {
		Expect(GetGPUDriverVersion("standard_nc6_v3")).To(Equal(datamodel.NvidiaCudaDriverVersion))
	})
	It("should use grid with nv v5", func() {
		Expect(GetGPUDriverVersion("standard_nv6ads_a10_v5")).To(Equal(datamodel.NvidiaGridDriverVersion))
		Expect(GetGPUDriverVersion("Standard_nv36adms_A10_V5")).To(Equal(datamodel.NvidiaGridDriverVersion))
	})
	// NV V1 SKUs were retired in September 2023, leaving this test just for safety
	It("should use cuda with nv v1", func() {
		Expect(GetGPUDriverVersion("standard_nv6")).To(Equal(datamodel.NvidiaCudaDriverVersion))
	})
})

var _ = Describe("GetGPUDriverType", func() {

	It("should use cuda with nc v3", func() {
		Expect(GetGPUDriverType("standard_nc6_v3")).To(Equal("cuda"))
	})
	It("should use grid with nv v5", func() {
		Expect(GetGPUDriverType("standard_nv6ads_a10_v5")).To(Equal("grid"))
		Expect(GetGPUDriverType("Standard_nv36adms_A10_V5")).To(Equal("grid"))
	})
	// NV V1 SKUs were retired in September 2023, leaving this test just for safety
	It("should use cuda with nv v1", func() {
		Expect(GetGPUDriverType("standard_nv6")).To(Equal("cuda"))
	})
})

var _ = Describe("GetAKSGPUImageSHA", func() {
	It("should use newest AKSGPUGridVersionSuffix with nv v5", func() {
		Expect(GetAKSGPUImageSHA("standard_nv6ads_a10_v5")).To(Equal(datamodel.AKSGPUGridVersionSuffix))
	})
	It("should use newest AKSGPUCudaVersionSuffix with non grid SKU", func() {
		Expect(GetAKSGPUImageSHA("standard_nc6_v3")).To(Equal(datamodel.AKSGPUCudaVersionSuffix))
	})
})

var _ = Describe("getLinuxNodeCSECommand", func() {
	var (
		templateGenerator *TemplateGenerator
		baseConfig        *datamodel.NodeBootstrappingConfiguration
	)

	BeforeEach(func() {
		templateGenerator = InitializeTemplateGenerator()
		agentPoolProfile := &datamodel.AgentPoolProfile{
			Name:   "nodepool1",
			OSType: datamodel.Linux,
			Distro: datamodel.AKSUbuntuContainerd2204Gen2,
		}
		baseConfig = &datamodel.NodeBootstrappingConfiguration{
			ContainerService: &datamodel.ContainerService{
				Location: "eastus",
				Properties: &datamodel.Properties{
					OrchestratorProfile: &datamodel.OrchestratorProfile{
						OrchestratorVersion: "1.29.0",
						KubernetesConfig: &datamodel.KubernetesConfig{
							ContainerRuntimeConfig: map[string]string{},
						},
					},
					HostedMasterProfile: &datamodel.HostedMasterProfile{
						FQDN: "test-cluster.hcp.eastus.azmk8s.io",
					},
					AgentPoolProfiles: []*datamodel.AgentPoolProfile{agentPoolProfile},
				},
			},
			AgentPoolProfile: agentPoolProfile,
			CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest,
			KubeletConfig:    map[string]string{},
		}
		baseConfig.K8sComponents = &datamodel.K8sComponents{}
		baseConfig.ContainerService.Properties.OrchestratorProfile.OrchestratorType = datamodel.Kubernetes
	})

	decodeCSEVars := func(cseCmd string) map[string]string {
		vars, err := getDecodedVarsFromCseCmd([]byte(cseCmd))
		Expect(err).NotTo(HaveOccurred())
		return vars
	}

	It("should generate a valid single-line CSE command", func() {
		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		// Verify it's a single line (no newlines)
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())
		// Verify it contains expected CSE components
		Expect(cseCmd).To(ContainSubstring("bash"))
	})

	It("should embed cloud-init status checks when custom data is enabled", func() {
		Expect(baseConfig.DisableCustomData).To(BeFalse())

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).To(ContainSubstring("CLOUD_INIT_STATUS_SCRIPT=\"/opt/azure/containers/cloud-init-status-check.sh\""))
		Expect(cseCmd).To(ContainSubstring("handleCloudInitStatus"))
		Expect(cseCmd).To(ContainSubstring("cloud-init status --wait"))
		Expect(cseCmd).To(ContainSubstring("cloudInitExitCode=$?"))
	})

	It("should handle configuration with custom kubelet config", func() {
		baseConfig.KubeletConfig = map[string]string{
			"--max-pods":                "110",
			"--pod-max-pids":            "-1",
			"--image-gc-high-threshold": "85",
		}

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars, err := getDecodedVarsFromCseCmd([]byte(cseCmd))
		Expect(err).NotTo(HaveOccurred())
		Expect(vars).To(HaveKey("KUBELET_FLAGS"))
		Expect(vars["KUBELET_FLAGS"]).To(Equal("--image-gc-high-threshold=85 --max-pods=110 --pod-max-pids=-1 "))
	})

	It("should handle different distros", func() {
		distros := []datamodel.Distro{
			datamodel.AKSUbuntuContainerd2204Gen2,
			datamodel.AKSCBLMarinerV2Gen2,
			datamodel.AKSAzureLinuxV2Gen2,
		}

		for _, distro := range distros {
			config, err := deepcopy.Anything(baseConfig)
			Expect(err).To(BeNil())
			typedConfig, ok := config.(*datamodel.NodeBootstrappingConfiguration)
			Expect(ok).To(BeTrue())
			typedConfig.AgentPoolProfile.Distro = distro

			cseCmd := templateGenerator.getLinuxNodeCSECommand(typedConfig)

			Expect(cseCmd).NotTo(BeEmpty())
			Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

			vars, decodeErr := getDecodedVarsFromCseCmd([]byte(cseCmd))
			Expect(decodeErr).NotTo(HaveOccurred())
			Expect(vars).To(HaveKeyWithValue("CSE_HELPERS_FILEPATH", "/opt/azure/containers/provision_source.sh"))
			Expect(vars).To(HaveKeyWithValue("CSE_DISTRO_HELPERS_FILEPATH", "/opt/azure/containers/provision_source_distro.sh"))
			Expect(vars).To(HaveKeyWithValue("CSE_DISTRO_INSTALL_FILEPATH", "/opt/azure/containers/provision_installs_distro.sh"))
		}
	})

	It("should handle GPU configuration", func() {
		baseConfig.EnableNvidia = true
		baseConfig.ConfigGPUDriverIfNeeded = true
		baseConfig.AgentPoolProfile.VMSize = "Standard_NC6s_v3"

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("GPU_NODE", "true"))
		Expect(vars).To(HaveKeyWithValue("CONFIG_GPU_DRIVER_IF_NEEDED", "true"))
		Expect(vars).To(HaveKeyWithValue("GPU_DRIVER_TYPE", "cuda"))
	})

	It("should handle custom cloud environment", func() {
		baseConfig.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
			Name:                    "akscustom",
			ResourceManagerEndpoint: "https://management.azure.fakecustomcloud/",
		}

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("IS_CUSTOM_CLOUD", "true"))
		Expect(vars).To(HaveKeyWithValue("TARGET_ENVIRONMENT", "akscustom"))
		Expect(strings.TrimSpace(vars["TARGET_CLOUD"])).To(Equal("AzureStackCloud"))
		Expect(vars["CUSTOM_ENV_JSON"]).NotTo(BeEmpty())
	})

	It("should handle TLS bootstrapping configuration", func() {
		baseConfig.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("TLS_BOOTSTRAP_TOKEN", "07401b.f395accd246ae52d"))
	})

	It("should handle kubelet serving certificate rotation", func() {
		baseConfig.KubeletConfig["--rotate-server-certificates"] = "true"

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION", "true"))
		Expect(vars["KUBELET_FLAGS"]).To(ContainSubstring("--rotate-server-certificates=true"))
	})

	It("should handle outbound type blocked configuration", func() {
		baseConfig.OutboundType = datamodel.OutboundTypeBlock

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("BLOCK_OUTBOUND_NETWORK", "true"))
		Expect(vars["OUTBOUND_COMMAND"]).To(BeEmpty())
	})

	It("should handle private egress configuration", func() {
		baseConfig.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
			PrivateEgress: &datamodel.PrivateEgress{
				Enabled:      true,
				ProxyAddress: "https://test-proxy.com",
			},
		}

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("PRIVATE_EGRESS_PROXY_ADDRESS", "https://test-proxy.com"))
	})

	It("should handle IMDS restriction configuration", func() {
		baseConfig.EnableIMDSRestriction = true
		baseConfig.InsertIMDSRestrictionRuleToMangleTable = true

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("ENABLE_IMDS_RESTRICTION", "true"))
		Expect(vars).To(HaveKeyWithValue("INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE", "true"))
	})

	It("should handle artifact streaming configuration", func() {
		baseConfig.EnableArtifactStreaming = true

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("ARTIFACT_STREAMING_ENABLED", "true"))
	})

	It("should handle custom CA trust certificates", func() {
		baseConfig.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
			CustomCATrustCerts: []string{"cert1", "cert2"},
		}

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("SHOULD_CONFIGURE_CUSTOM_CA_TRUST", "true"))
		Expect(vars).To(HaveKeyWithValue("CUSTOM_CA_TRUST_COUNT", "2"))
		Expect(vars).To(HaveKeyWithValue("CUSTOM_CA_CERT_0", "cert1"))
		Expect(vars).To(HaveKeyWithValue("CUSTOM_CA_CERT_1", "cert2"))
	})

	It("should handle custom Linux OS config", func() {
		netCoreSomaxconn := int32(16384)
		baseConfig.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
			Sysctls: &datamodel.SysctlConfig{
				NetCoreSomaxconn: &netCoreSomaxconn,
			},
			TransparentHugePageEnabled: "never",
		}

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("SHOULD_CONFIG_TRANSPARENT_HUGE_PAGE", "true"))
		Expect(vars).To(HaveKeyWithValue("THP_ENABLED", "never"))
		sysctlContentEncoded := vars["SYSCTL_CONTENT"]
		Expect(sysctlContentEncoded).NotTo(BeEmpty())
		decodedSysctl, decodeErr := base64.StdEncoding.DecodeString(sysctlContentEncoded)
		Expect(decodeErr).NotTo(HaveOccurred())
		Expect(string(decodedSysctl)).To(ContainSubstring("net.core.somaxconn"))
	})

	It("should handle SSH configuration", func() {
		baseConfig.SSHStatus = datamodel.SSHOff

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("DISABLE_SSH", "true"))
		Expect(vars).To(HaveKeyWithValue("DISABLE_PUBKEY_AUTH", "false"))
	})

	It("should handle FIPS enabled configuration", func() {
		baseConfig.FIPSEnabled = true

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("NEEDS_CGROUPV2", "true"))
	})

	It("should panic when template processing fails", func() {
		// Create invalid config that will cause template processing to fail
		invalidConfig := &datamodel.NodeBootstrappingConfiguration{
			AgentPoolProfile: nil, // This should cause an error
		}

		Expect(func() {
			templateGenerator.getLinuxNodeCSECommand(invalidConfig)
		}).To(Panic())
	})

	It("should handle credential provider configuration", func() {
		if baseConfig.K8sComponents == nil {
			baseConfig.K8sComponents = &datamodel.K8sComponents{}
		}
		baseConfig.K8sComponents.LinuxCredentialProviderURL = "https://example.com/provider.tar.gz"
		baseConfig.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
		baseConfig.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("CREDENTIAL_PROVIDER_DOWNLOAD_URL", "https://example.com/provider.tar.gz"))
		Expect(vars["KUBELET_FLAGS"]).To(ContainSubstring("--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml"))
		Expect(vars["KUBELET_FLAGS"]).To(ContainSubstring("--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider"))
	})

	It("should handle multiple kubernetes versions", func() {
		versions := []string{"1.28.0", "1.29.0", "1.30.0"}

		for _, version := range versions {
			config, err := deepcopy.Anything(baseConfig)
			Expect(err).To(BeNil())
			typedConfig, ok := config.(*datamodel.NodeBootstrappingConfiguration)
			Expect(ok).To(BeTrue())
			typedConfig.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version

			cseCmd := templateGenerator.getLinuxNodeCSECommand(typedConfig)

			Expect(cseCmd).NotTo(BeEmpty())
			Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

			vars := decodeCSEVars(cseCmd)
			Expect(vars).To(HaveKeyWithValue("KUBERNETES_VERSION", version))
		}
	})

	It("should handle MIG GPU configuration", func() {
		baseConfig.GPUInstanceProfile = "MIG7g"
		baseConfig.ConfigGPUDriverIfNeeded = true
		baseConfig.EnableNvidia = true
		baseConfig.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("GPU_NODE", "true"))
		Expect(vars).To(HaveKeyWithValue("CONFIG_GPU_DRIVER_IF_NEEDED", "true"))
		Expect(vars).To(HaveKeyWithValue("GPU_INSTANCE_PROFILE", "MIG7g"))
	})

	It("should handle disable unattended upgrades", func() {
		baseConfig.DisableUnattendedUpgrades = true

		cseCmd := templateGenerator.getLinuxNodeCSECommand(baseConfig)

		Expect(cseCmd).NotTo(BeEmpty())
		Expect(strings.Contains(cseCmd, "\n")).To(BeFalse())

		vars := decodeCSEVars(cseCmd)
		Expect(vars).To(HaveKeyWithValue("ENABLE_UNATTENDED_UPGRADES", "false"))
	})
})

var _ = Describe("cloudInitToButane", func() {
	checkForUnit := func(butane flatcar1_1.Config) {
		Expect(butane.Systemd.Units).To(HaveLen(2))
		var unit = butane.Systemd.Units[0]
		Expect(unit.Name).To(Equal("ignition-bootcmds.service"))
		Expect(*unit.Contents).To(ContainSubstring("/etc/ignition-bootcmds.sh"))
	}

	It("should convert bootcmds to a systemd unit and shell script", func() {
		var config = cloudInit{BootCommands: []string{"echo hello world", "ls 'some dir'"}}
		var butane = cloudInitToButane(config)
		checkForUnit(butane)
		Expect(butane.Storage.Files).To(HaveLen(1))
		var file = butane.Storage.Files[0]
		Expect(file.Path).To(Equal(ignitionFilesTarPath))
		Expect(file.Contents.Source).NotTo(BeNil())
		tarball, err := decodeButaneResource(file.Contents)
		Expect(err).To(BeNil())
		files, err := decodeTarFiles(tarball)
		Expect(err).To(BeNil())
		var ignitionBootcmdScript *decodedValue
		for _, f := range files {
			if f.path == ignitionBootcmdScriptPath {
				ignitionBootcmdScript = f.decodedValue
			}
		}
		Expect(ignitionBootcmdScript).To(Not(BeNil()))
		Expect(ignitionBootcmdScript.value).To(Equal("#!/bin/sh\necho hello world\nls 'some dir'"))
		Expect(butane.Systemd.Units).NotTo(BeEmpty())
		found := false
		for _, unit := range butane.Systemd.Units {
			if unit.Name == ignitionTarUnitName {
				found = true
				Expect(unit.Contents).NotTo(BeNil())
				Expect(*unit.Contents).To(ContainSubstring(ignitionFilesTarPath))
				break
			}
		}
		Expect(found).To(BeTrue())
	})

	It("should decode gzip-encoded write_files and preserve permissions", func() {
		// gzip content is stored as raw bytes via YAML !!binary, so use the raw gzip buffer.
		plainText := "hello from gzip"
		gzipped := getGzippedBufferFromBytes([]byte(plainText))
		var config = cloudInit{WriteFiles: []cloudInitWriteFile{
			{
				Path:        "/etc/test-gzip",
				Permissions: "0644",
				Encoding:    encodingGZIP,
				Content:     string(gzipped),
			},
		}}
		var butane = cloudInitToButane(config)
		Expect(butane.Storage.Files).To(HaveLen(1))
		var file = butane.Storage.Files[0]
		tarball, err := decodeButaneResource(file.Contents)
		Expect(err).To(BeNil())
		files, err := decodeTarFiles(tarball)
		Expect(err).To(BeNil())
		var decoded *decodedValue
		for _, f := range files {
			if f.path == "/etc/test-gzip" {
				decoded = f.decodedValue
			}
		}
		Expect(decoded).NotTo(BeNil())
		Expect(decoded.value).To(Equal(plainText))
		Expect(decoded.mode).To(Equal(int64(0o644)))
	})

	It("should decode base64-encoded write_files and preserve permissions", func() {
		plainText := "hello from base64"
		encoded := base64.StdEncoding.EncodeToString([]byte(plainText))
		var config = cloudInit{WriteFiles: []cloudInitWriteFile{
			{
				Path:        "/etc/test-base64",
				Permissions: "0600",
				Encoding:    "base64",
				Content:     encoded,
			},
		}}
		var butane = cloudInitToButane(config)
		Expect(butane.Storage.Files).To(HaveLen(1))
		var file = butane.Storage.Files[0]
		tarball, err := decodeButaneResource(file.Contents)
		Expect(err).To(BeNil())
		files, err := decodeTarFiles(tarball)
		Expect(err).To(BeNil())
		var decoded *decodedValue
		for _, f := range files {
			if f.path == "/etc/test-base64" {
				decoded = f.decodedValue
			}
		}
		Expect(decoded).NotTo(BeNil())
		Expect(decoded.value).To(Equal(plainText))
		Expect(decoded.mode).To(Equal(int64(0o600)))
	})

	It("should create a system unit but not a shell script with no bootcmds", func() {
		var config = cloudInit{BootCommands: []string{}}
		var butane = cloudInitToButane(config)
		checkForUnit(butane)
		Expect(butane.Storage.Files).To(BeEmpty())
		Expect(butane.Systemd.Units).NotTo(BeEmpty())
		found := false
		for _, unit := range butane.Systemd.Units {
			if unit.Name == ignitionTarUnitName {
				found = true
				Expect(unit.Contents).NotTo(BeNil())
				Expect(*unit.Contents).To(ContainSubstring(ignitionFilesTarPath))
				break
			}
		}
		Expect(found).To(BeTrue())
	})
})

func decodeButaneResource(resource base0_5.Resource) ([]byte, error) {
	if resource.Source == nil {
		return nil, fmt.Errorf("resource source is nil")
	}
	decodeddata, err := dataurl.DecodeString(*resource.Source)
	if err != nil {
		return nil, err
	}
	contents := decodeddata.Data
	if resource.Compression != nil && *resource.Compression == encodingGZIP {
		contents, err = getGzipDecodedValue(contents)
		if err != nil {
			return nil, err
		}
	}
	return contents, nil
}
