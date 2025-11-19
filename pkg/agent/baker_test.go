package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/barkimedes/go-deepcopy"
	ign3_4 "github.com/coreos/ignition/v2/config/v3_4/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/vincent-petithory/dataurl"
	"gopkg.in/yaml.v3"
)

func generateTestData() bool {
	return os.Getenv("GENERATE_TEST_DATA") == "true"
}

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

type nodeBootstrappingOutput struct {
	customData string
	cseCmd     string
	files      map[string]*decodedValue
	vars       map[string]string
}

type decodedValue struct {
	encoding cseVariableEncoding
	value    string
}

type cseVariableEncoding string

const (
	cseVariableEncodingGzip cseVariableEncoding = "gzip"
)

type outputValidator func(*nodeBootstrappingOutput)

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
			// Expect an error if LocalDNSProfile is nil and GenerateLocalDNSCoreFile is invoked somehow.
			It("returns an error when LocalDNSProfile is nil", func() {
				config.AgentPoolProfile.LocalDNSProfile = nil
				_, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("localdns profile is nil"))
			})

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

			// Expect an error from GenerateLocalDNSCoreFile if it is invoked when EnableLocalDNS is set to false.
			It("returns an error when EnableLocalDNS is set to false", func() {
				config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
					EnableLocalDNS:       false,
					CPULimitInMilliCores: to.Int32Ptr(2008),
					MemoryLimitInMB:      to.Int32Ptr(128),
					VnetDNSOverrides:     nil,
					KubeDNSOverrides:     nil,
				}
				_, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("EnableLocalDNS is set to false, corefile will not be generated"))
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
				localDNSCoreFileGzippedBase64Encoded, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Encoded).ToNot(BeEmpty())

				// Decode the gzipped base64 encoded string.
				localDNSCoreFileGzippedBase64Decoded, err := getBase64DecodedValue([]byte(localDNSCoreFileGzippedBase64Encoded))
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Decoded).ToNot(BeEmpty())

				// Decompress the gzipped data.
				localDNSCorefile, err := getGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
				Expect(err).To(BeNil())
				Expect(localDNSCorefile).ToNot(BeEmpty())
				Expect(localDNSCorefile).To(ContainSubstring(expectedlocalDNSCorefileWithoutOverrides))
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
				localDNSCoreFileGzippedBase64Encoded, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Encoded).ToNot(BeEmpty())

				// Decode the gzipped base64 encoded string.
				localDNSCoreFileGzippedBase64Decoded, err := getBase64DecodedValue([]byte(localDNSCoreFileGzippedBase64Encoded))
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Decoded).ToNot(BeEmpty())

				// Decompress the gzipped data.
				localDNSCorefile, err := getGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
				Expect(err).To(BeNil())
				Expect(localDNSCorefile).ToNot(BeEmpty())
				Expect(localDNSCorefile).To(ContainSubstring(expectedlocalDNSCorefileWithoutOverrides))
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
				localDNSCoreFileGzippedBase64Encoded, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Encoded).ToNot(BeEmpty())

				// Decode the gzipped base64 encoded string.
				localDNSCoreFileGzippedBase64Decoded, err := getBase64DecodedValue([]byte(localDNSCoreFileGzippedBase64Encoded))
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Decoded).ToNot(BeEmpty())

				// Decompress the gzipped data.
				localDNSCorefile, err := getGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
				Expect(err).To(BeNil())
				Expect(localDNSCorefile).ToNot(BeEmpty())

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
				Expect(localDNSCorefile).To(ContainSubstring(expectedlocalDNSCorefile))
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
				localDNSCoreFileGzippedBase64Encoded, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, localDNSCoreFileTemplateString)
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Encoded).ToNot(BeEmpty())

				// Decode the gzipped base64 encoded string.
				localDNSCoreFileGzippedBase64Decoded, err := getBase64DecodedValue([]byte(localDNSCoreFileGzippedBase64Encoded))
				Expect(err).To(BeNil())
				Expect(localDNSCoreFileGzippedBase64Decoded).ToNot(BeEmpty())

				// Decompress the gzipped data.
				localDNSCorefile, err := getGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
				Expect(err).To(BeNil())
				Expect(localDNSCorefile).ToNot(BeEmpty())

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
				Expect(localDNSCorefile).To(ContainSubstring(expectedlocalDNSCorefile))
			})
		})
	})
	// ------------------------------- End of tests related to Localdns ---------------------------------------

	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, configUpdator func(*datamodel.NodeBootstrappingConfiguration),
		validator outputValidator) {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: k8sVersion,
					KubernetesConfig:    &datamodel.KubernetesConfig{},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					DNSPrefix: "uttestdom",
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "agent2",
						VMSize:              "Standard_DS1_v2",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Linux,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet1",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						Distro:              datamodel.AKSUbuntuContainerd2204Gen2,
					},
				},
				LinuxProfile: &datamodel.LinuxProfile{
					AdminUsername: "azureuser",
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "ClientID",
					Secret:   "Secret",
				},
			},
		}
		cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
			KeyData: string("testsshkey"),
		}}

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]

		k8sComponents := &datamodel.K8sComponents{}

		if IsKubernetesVersionGe(k8sVersion, "1.29.0") {
			k8sComponents.WindowsCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", k8sVersion, k8sVersion) //nolint:lll
			k8sComponents.LinuxCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz", k8sVersion, k8sVersion)     //nolint:lll
		}

		kubeletConfig := map[string]string{
			"--address":                           "0.0.0.0",
			"--pod-manifest-path":                 "/etc/kubernetes/manifests",
			"--cloud-provider":                    "azure",
			"--cloud-config":                      "/etc/kubernetes/azure.json",
			"--azure-container-registry-config":   "/etc/kubernetes/azure.json",
			"--cluster-domain":                    "cluster.local",
			"--cluster-dns":                       "10.0.0.10",
			"--cgroups-per-qos":                   "true",
			"--tls-cert-file":                     "/etc/kubernetes/certs/kubeletserver.crt",
			"--tls-private-key-file":              "/etc/kubernetes/certs/kubeletserver.key",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
			"--max-pods":                          "110",
			"--node-status-update-frequency":      "10s",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--event-qps":                         "0",
			"--pod-max-pids":                      "-1",
			"--enforce-node-allocatable":          "pods",
			"--streaming-connection-idle-timeout": "4h0m0s",
			"--rotate-certificates":               "true",
			"--read-only-port":                    "10255",
			"--protect-kernel-defaults":           "true",
			"--resolv-conf":                       "/etc/resolv.conf",
			"--anonymous-auth":                    "false",
			"--client-ca-file":                    "/etc/kubernetes/certs/ca.crt",
			"--authentication-token-webhook":      "true",
			"--authorization-mode":                "Webhook",
			"--eviction-hard":                     "memory.available<750Mi,nodefs.available<10%,nodefs.inodesFree<5%",
			"--feature-gates":                     "RotateKubeletServerCertificate=true,a=b,PodPriority=true,x=y",
			"--system-reserved":                   "cpu=2,memory=1Gi",
			"--kube-reserved":                     "cpu=100m,memory=1638Mi",
			"--container-log-max-size":            "50M",
		}

		config := &datamodel.NodeBootstrappingConfiguration{
			ContainerService:              cs,
			CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
			K8sComponents:                 k8sComponents,
			AgentPoolProfile:              agentPool,
			TenantID:                      "tenantID",
			SubscriptionID:                "subID",
			ResourceGroupName:             "resourceGroupName",
			UserAssignedIdentityClientID:  "userAssignedID",
			ConfigGPUDriverIfNeeded:       true,
			EnableGPUDevicePluginIfNeeded: false,
			EnableKubeletConfigFile:       false,
			EnableNvidia:                  false,
			FIPSEnabled:                   false,
			KubeletConfig:                 kubeletConfig,
			PrimaryScaleSetName:           "aks-agent2-36873793-vmss",
			IsARM64:                       false,
			DisableUnattendedUpgrades:     false,
			SSHStatus:                     datamodel.SSHUnspecified,
			SIGConfig: datamodel.SIGConfig{
				TenantID:       "tenantID",
				SubscriptionID: "subID",
				Galleries: map[string]datamodel.SIGGalleryConfig{
					"AKSUbuntu": {
						GalleryName:   "aksubuntu",
						ResourceGroup: "resourcegroup",
					},
					"AKSCBLMariner": {
						GalleryName:   "akscblmariner",
						ResourceGroup: "resourcegroup",
					},
					"AKSAzureLinux": {
						GalleryName:   "aksazurelinux",
						ResourceGroup: "resourcegroup",
					},
					"AKSWindows": {
						GalleryName:   "AKSWindows",
						ResourceGroup: "AKS-Windows",
					},
					"AKSUbuntuEdgeZone": {
						GalleryName:   "AKSUbuntuEdgeZone",
						ResourceGroup: "AKS-Ubuntu-EdgeZone",
					},
					"AKSFlatcar": {
						GalleryName:   "aksflatcar",
						ResourceGroup: "resourcegroup",
					},
				},
			},
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// !!! WARNING !!!
		// avoid mutation of the original config -- both functions mutate input.
		// GetNodeBootstrappingPayload mutates the input so it's not the same as what gets passed to GetNodeBootstrappingCmd which causes bugs.
		// unit tests should always rely on un-mutated copies of the base config.
		configCustomDataInput, err := deepcopy.Anything(config)
		Expect(err).To(BeNil())

		configCseInput, err := deepcopy.Anything(config)
		Expect(err).To(BeNil())

		// customData
		ab, err := NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err := ab.GetNodeBootstrapping(
			context.Background(),
			configCustomDataInput.(*datamodel.NodeBootstrappingConfiguration), //nolint:errcheck // this code been writen before linter was added
		)
		Expect(err).To(BeNil())

		var customDataBytes []byte
		if config.AgentPoolProfile.IsWindows() || config.IsFlatcar() {
			customDataBytes, err = base64.StdEncoding.DecodeString(nodeBootstrapping.CustomData)
			Expect(err).To(BeNil())
		} else {
			var zippedDataBytes []byte
			// try to unzip the bytes. If this fails then the custom data was not zipped. And it should be due to customdata size limitations.
			zippedDataBytes, err = base64.StdEncoding.DecodeString(nodeBootstrapping.CustomData)
			Expect(err).To(BeNil())
			customDataBytes, err = getGzipDecodedValue(zippedDataBytes)
			Expect(err).To(BeNil())
		}

		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		Expect(err).To(BeNil())
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		ab, err = NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err = ab.GetNodeBootstrapping(
			context.Background(),
			configCseInput.(*datamodel.NodeBootstrappingConfiguration), //nolint:errcheck // this code been writen before linter was added
		)
		Expect(err).To(BeNil())
		cseCommand := nodeBootstrapping.CSE

		if generateTestData() {
			err = os.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
			Expect(err).To(BeNil())
		}
		expectedCSECommand, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		Expect(err).To(BeNil())
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

		files, err := getDecodedFilesFromCustomdata(customDataBytes)
		Expect(err).To(BeNil())

		vars, err := getDecodedVarsFromCseCmd([]byte(cseCommand))
		Expect(err).To(BeNil())

		result := &nodeBootstrappingOutput{
			customData: customData,
			cseCmd:     cseCommand,
			files:      files,
			vars:       vars,
		}

		if validator != nil {
			validator(result)
		}

	},
		Entry("AKSUbuntu2204 with kubelet serving certificate rotation implicitly disabled", "AKSUbuntu2204+ImplicitlyDisableKubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation explicitly disabled", "AKSUbuntu2204+DisableKubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "false"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--rotate-server-certificates=false")).To(BeTrue())
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation enabled", "AKSUbuntu2204+KubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "true"
				config.KubeletConfig["--tls-cert-file"] = "cert.crt"
				config.KubeletConfig["--tls-private-key-file"] = "cert.key"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("true"))
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--rotate-server-certificates=true")).To(BeTrue())
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation disabled and custom kubelet config",
			"AKSUbuntu2204+DisableKubeletServingCertificateRotation+CustomKubeletConfig", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--rotate-server-certificates"] = "false"
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("false"))
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).ToNot(ContainSubstring("serverTLSBootstrap")) // because of: "bool `json:"serverTLSBootstrap,omitempty"`"
			}),

		Entry("AKSUbuntu2204 with kubelet serving certificate rotation enabled and custom kubelet config",
			"AKSUbuntu2204+KubeletServingCertificateRotation+CustomKubeletConfig", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--rotate-server-certificates"] = "true"
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_KUBELET_SERVING_CERTIFICATE_ROTATION"]).To(Equal("true"))
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).To(ContainSubstring(`"serverTLSBootstrap": true`))
			}),

		Entry("Mariner v2 with kata", "MarinerV2+Kata", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud+USSec", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "ussecwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud+USNat", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "usnatwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud+USSec", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "ussecwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("Mariner v2 with custom cloud", "MarinerV2+CustomCloud+USNat", "1.23.8", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Mariner"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "usnatwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),

		Entry("AzureLinux v2 with kata", "AzureLinuxV2+Kata", "1.28.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "AzureLinux"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("AzureLinux v3 with kata", "AzureLinuxV3+Kata", "1.28.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "AzureLinux"
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),

		Entry("Mariner v2 with DisableUnattendedUpgrades=true", "Marinerv2+DisableUnattendedUpgrades=true", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("Mariner v2 with DisableUnattendedUpgrades=false", "Marinerv2+DisableUnattendedUpgrades=false", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("Mariner v2 with kata and DisableUnattendedUpgrades=true", "Marinerv2+Kata+DisableUnattendedUpgrades=true", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("Mariner v2 with kata and DisableUnattendedUpgrades=false", "Marinerv2+Kata+DisableUnattendedUpgrades=false", "1.23.8",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "Mariner"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSCBLMarinerV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v2 with DisableUnattendedUpgrades=true", "AzureLinuxv2+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v2 with DisableUnattendedUpgrades=false", "AzureLinuxv2+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v2 with kata and DisableUnattendedUpgrades=true", "AzureLinuxv2+Kata+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v2 with kata and DisableUnattendedUpgrades=false", "AzureLinuxv2+Kata+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV2Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AzureLinux v3 with kata and DisableUnattendedUpgrades=true", "AzureLinuxV3+Kata+DisableUnattendedUpgrades=true", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = true
			}, nil),

		Entry("AzureLinux v3 with kata and DisableUnattendedUpgrades=false", "AzureLinuxV3+Kata+DisableUnattendedUpgrades=false", "1.28.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.OSSKU = "AzureLinux"
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.DisableUnattendedUpgrades = false
			}, nil),

		Entry("AKSUbuntu2204 with outbound type blocked", "AKSUbuntu2204+OutboundTypeBlocked", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = datamodel.OutboundTypeBlock
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("true"))
		}),

		Entry("AKSUbuntu2204 with outbound type none", "AKSUbuntu2204+OutboundTypeNone", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = datamodel.OutboundTypeNone
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("true"))
		}),

		Entry("AKSUbuntu2204 with no outbound type", "AKSUbuntu2204+OutboundTypeNil", "1.25.6", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OutboundType = ""
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["BLOCK_OUTBOUND_NETWORK"]).To(Equal("false"))
		}),

		Entry("AKSUbuntu2204 with SerializeImagePulls=false and k8s 1.31", "AKSUbuntu2204+SerializeImagePulls", "1.31.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--serialize-image-pulls"] = "false"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--serialize-image-pulls=false")).To(BeTrue())
		}),
		Entry("AKSUbuntu2204 w/o artifact streaming", "AKSUbuntu2204+NoArtifactStreaming", "1.25.7", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableArtifactStreaming = false
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
		},
			func(o *nodeBootstrappingOutput) {

				Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
				Expect(err).To(BeNil())
				expectedOverlaybdConfig := `[plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"`
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(expectedOverlaybdConfig))
				expectedOverlaybdPlugin := `[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"`
				Expect(containerdConfigFileContent).NotTo(ContainSubstring(expectedOverlaybdPlugin))
			},
		),
		Entry("AKSUbuntu2204 VHD, cgroupv2", "AKSUbuntu2204+cgroupv2", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
		}, nil),
		Entry("AKSUbuntu2204 with containerd and CDI enabled", "AKSUbuntu2204+Containerd+CDI", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
			config.KubeletConfig = map[string]string{}
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["CONTAINERD_CONFIG_CONTENT"]).NotTo(BeEmpty())
			containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_CONTENT"]))
			Expect(err).To(BeNil())
			Expect(containerdConfigFileContent).To(ContainSubstring("enable_cdi = true"))
		}),
		Entry("AKSUbuntu2204 containerd with multi-instance GPU", "AKSUbuntu2204+Containerd+MIG", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				// the purpose of this unit test is to ensure the containerd config
				// does not use the nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.GPUInstanceProfile = "MIG7g"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
`

				Expect(containerdConfigFileContent).To(Equal(expectedShimConfig))
			}),
		Entry("AKSUbuntu2204 containerd with multi-instance GPU and artifact streaming", "AKSUbuntu2204+Containerd+MIG+ArtifactStreaming", "1.19.13",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.7"
				config.EnableArtifactStreaming = true
				config.AgentPoolProfile.VMSize = "Standard_ND96asr_v4"
				// the purpose of this unit test is to ensure the containerd config
				// does not use the nvidia container runtime when skipping the
				// GPU driver install, since it will fail to run even non-GPU
				// pods, as it will not be installed.
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.GPUInstanceProfile = "MIG7g"
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]).NotTo(BeEmpty())
				containerdConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["CONTAINERD_CONFIG_NO_GPU_CONTENT"]))
				Expect(err).To(BeNil())
				expectedShimConfig := `version = 2
oom_score = -999
[plugins."io.containerd.grpc.v1.cri"]
  sandbox_image = ""
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "overlaybd"
    disable_snapshot_annotations = false
    default_runtime_name = "runc"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
      BinaryName = "/usr/bin/runc"
      SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
      runtime_type = "io.containerd.runc.v2"
    [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted.options]
      BinaryName = "/usr/bin/runc"
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
  [plugins."io.containerd.grpc.v1.cri".registry.headers]
    X-Meta-Source-Client = ["azure/aks"]
[metrics]
  address = "0.0.0.0:10257"
[proxy_plugins]
  [proxy_plugins.overlaybd]
    type = "snapshot"
    address = "/run/overlaybd-snapshotter/overlaybd.sock"
`

				Expect(containerdConfigFileContent).To(Equal(expectedShimConfig))
			}),
		Entry("AKSUbuntu2204 with NVIDIA Device Plugin enabled", "AKSUbuntu2204+Containerd+DevicePlugin", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.EnableGPUDevicePluginIfNeeded = true
			}, func(o *nodeBootstrappingOutput) {
				// Verify device plugin is enabled
				Expect(o.vars["ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED"]).To(Equal("true"))
				// Verify GPU node is set
				Expect(o.vars["GPU_NODE"]).To(Equal("true"))
				// Verify GPU driver configuration is enabled
				Expect(o.vars["CONFIG_GPU_DRIVER_IF_NEEDED"]).To(Equal("true"))
			}),
		Entry("AKSUbuntu2204 with ManagedGPUExperienceAFECEnabled", "AKSUbuntu2204+ManagedGPUExperienceAFEC", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.EnableGPUDevicePluginIfNeeded = true
				config.ManagedGPUExperienceAFECEnabled = true
			}, func(o *nodeBootstrappingOutput) {
				// Verify ManagedGPUExperienceAFECEnabled is set
				Expect(o.vars["MANAGED_GPU_EXPERIENCE_AFEC_ENABLED"]).To(Equal("true"))
				// Verify other GPU settings are also correct
				Expect(o.vars["GPU_NODE"]).To(Equal("true"))
				Expect(o.vars["ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED"]).To(Equal("true"))
			}),
		Entry("AKSUbuntu2204 with ManagedGPUExperienceAFECEnabled disabled", "AKSUbuntu2204+ManagedGPUExperienceAFEC+Disabled", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2204
				config.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				config.EnableNvidia = true
				config.ConfigGPUDriverIfNeeded = true
				config.EnableGPUDevicePluginIfNeeded = true
				config.ManagedGPUExperienceAFECEnabled = false
			}, func(o *nodeBootstrappingOutput) {
				// Verify ManagedGPUExperienceAFECEnabled is disabled
				Expect(o.vars["MANAGED_GPU_EXPERIENCE_AFEC_ENABLED"]).To(Equal("false"))
				// Verify other GPU settings are still correct
				Expect(o.vars["GPU_NODE"]).To(Equal("true"))
				Expect(o.vars["ENABLE_GPU_DEVICE_PLUGIN_IF_NEEDED"]).To(Equal("true"))
			}),
		Entry("CustomizedImage VHD should not have provision_start.sh", "CustomizedImage", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImage
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("CustomizedImageKata VHD should not have provision_start.sh", "CustomizedImageKata", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImageKata
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("CustomizedImageLinuxGuard VHD should not have provision_start.sh", "CustomizedImageLinuxGuard", "1.24.2",
			func(c *datamodel.NodeBootstrappingConfiguration) {
				c.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
					ContainerRuntime: datamodel.Containerd,
				}
				c.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.CustomizedImageLinuxGuard
			}, func(o *nodeBootstrappingOutput) {
				_, exist := o.files["/opt/azure/containers/provision_start.sh"]

				Expect(exist).To(BeFalse())
			},
		),
		Entry("Flatcar", "Flatcar", "1.31.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = datamodel.OSSKUFlatcar
			config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSFlatcarGen2
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
		}, nil),
		Entry("Flatcar with custom cloud", "Flatcar+CustomCloud", "1.32.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = datamodel.OSSKUFlatcar
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),
		Entry("Flatcar with custom cloud", "Flatcar+CustomCloud+USSec", "1.33.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.OSSKU = "Flatcar"
			config.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig = &datamodel.KubernetesConfig{
				ContainerRuntime: datamodel.Containerd,
			}
			config.ContainerService.Location = "ussecwest"
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),
		Entry("AKSUbuntu2204 DisableSSH with enabled ssh", "AKSUbuntu2204+SSHStatusOn", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.SSHOn
		}, nil),
		Entry("AKSUbuntu2204 DisableSSH with disabled ssh", "AKSUbuntu2204+SSHStatusOff", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.SSHOff
		}, nil),
		Entry("AKSUbuntu2204 with Entra ID SSH", "AKSUbuntu2204+SSHStatusEntraID", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.SSHStatus = datamodel.EntraIDSSH
		}, nil),
		Entry("AKSUbuntu2204 in China", "AKSUbuntu2204+China", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "AzureChinaCloud",
			}
			config.ContainerService.Location = "chinaeast2"
		}, nil),
		Entry("AKSUbuntu2204 custom cloud", "AKSUbuntu2204+CustomCloud", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
		}, nil),
		Entry("AKSUbuntu2204 custom cloud", "AKSUbuntu2204+CustomCloud+USSec", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
			config.ContainerService.Location = "ussecwest"
		}, nil),
		Entry("AKSUbuntu2204 custom cloud", "AKSUbuntu2204+CustomCloud+USNat", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name: "akscustom",
			}
			config.ContainerService.Location = "usnatwest"
		}, nil),
		Entry("AKSUbuntu2204 OOT credentialprovider", "AKSUbuntu2204+ootcredentialprovider", "1.29.10", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
			config.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml")).To(BeTrue())
			Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider")).To(BeTrue())
		}),
		Entry("AKSUbuntu2204 custom cloud and OOT credentialprovider", "AKSUbuntu2204+CustomCloud+ootcredentialprovider", "1.29.10",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
					Name:                         "akscustom",
					McrURL:                       "mcr.microsoft.fakecustomcloud",
					RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
					ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
					PublishSettingsURL:           "",
					ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
					ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
					ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
					GalleryEndpoint:              "",
					KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
					GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
					ServiceBusEndpoint:           "",
					BatchManagementEndpoint:      "",
					StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
					SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
					TrafficManagerDNSSuffix:      "",
					KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
					ServiceBusEndpointSuffix:     "",
					ServiceManagementVMDNSSuffix: "",
					ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
					ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
					CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
					TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
					ResourceIdentifiers: datamodel.ResourceIdentifiers{
						Graph:               "",
						KeyVault:            "",
						Datalake:            "",
						Batch:               "",
						OperationalInsights: "",
						Storage:             "",
					},
				}
				config.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				config.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			}, func(o *nodeBootstrappingOutput) {

				Expect(o.vars["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"]).NotTo(BeEmpty())
				Expect(o.vars["AKS_CUSTOM_CLOUD_CONTAINER_REGISTRY_DNS_SUFFIX"]).To(Equal(".azurecr.microsoft.fakecustomcloud"))

				Expect(o.vars["KUBELET_FLAGS"]).NotTo(BeEmpty())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-config=/var/lib/kubelet/credential-provider-config.yaml")).To(BeTrue())
				Expect(strings.Contains(o.vars["KUBELET_FLAGS"], "--image-credential-provider-bin-dir=/var/lib/kubelet/credential-provider")).To(BeTrue())
			}),
		Entry("AKSUbuntu2204 with custom kubeletConfig and osConfig", "AKSUbuntu2204+CustomKubeletConfig+CustomLinuxOSConfig", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				netIpv4TcpTwReuse := true
				failSwapOn := false
				var swapFileSizeMB int32 = 1500
				var netCoreSomaxconn int32 = 1638499
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
					SeccompDefault:        to.BoolPtr(true),
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetCoreSomaxconn:             &netCoreSomaxconn,
						NetCoreRmemDefault:           to.Int32Ptr(456000),
						NetCoreWmemDefault:           to.Int32Ptr(89000),
						NetIpv4TcpTwReuse:            &netIpv4TcpTwReuse,
						NetIpv4IpLocalPortRange:      "32768 65400",
						NetIpv4TcpMaxSynBacklog:      to.Int32Ptr(1638498),
						NetIpv4NeighDefaultGcThresh1: to.Int32Ptr(10001),
					},
					TransparentHugePageEnabled: "never",
					TransparentHugePageDefrag:  "defer+madvise",
					SwapFileSizeMB:             &swapFileSizeMB,
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: "75000",
						NoFile:          "1048",
					},
				}
			}, func(o *nodeBootstrappingOutput) {
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				var kubeletConfigFile datamodel.AKSKubeletConfiguration
				err = json.Unmarshal([]byte(kubeletConfigFileContent), &kubeletConfigFile)
				Expect(err).To(BeNil())
				Expect(kubeletConfigFile.SeccompDefault).To(Equal(to.BoolPtr(true)))

				sysctlContent, err := getBase64DecodedValue([]byte(o.vars["SYSCTL_CONTENT"]))
				Expect(err).To(BeNil())
				// assert defaults for gc_thresh2 and gc_thresh3
				// assert custom values for all others.
				Expect(sysctlContent).To(ContainSubstring("net.core.somaxconn=1638499"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.tcp_max_syn_backlog=1638498"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh1=10001"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh2=8192"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.neigh.default.gc_thresh3=16384"))
				Expect(sysctlContent).To(ContainSubstring("net.ipv4.ip_local_reserved_ports=65330"))

				Expect(o.vars["SHOULD_CONFIG_CONTAINERD_ULIMITS"]).To(Equal("true"))
				containerdUlimitContent := o.vars["CONTAINERD_ULIMITS"]
				Expect(containerdUlimitContent).To(ContainSubstring("LimitNOFILE=1048"))
				Expect(containerdUlimitContent).To(ContainSubstring("LimitMEMLOCK=75000"))
			}),
		Entry("AKSUbuntu2204 with k8s 1.31 and custom kubeletConfig and serializeImagePull flag", "AKSUbuntu2204+CustomKubeletConfig+SerializeImagePulls", "1.31.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableKubeletConfigFile = false
				failSwapOn := false
				config.KubeletConfig["--serialize-image-pulls"] = "false"
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = &datamodel.CustomKubeletConfig{
					CPUManagerPolicy:      "static",
					CPUCfsQuota:           to.BoolPtr(false),
					CPUCfsQuotaPeriod:     "200ms",
					ImageGcHighThreshold:  to.Int32Ptr(90),
					ImageGcLowThreshold:   to.Int32Ptr(70),
					TopologyManagerPolicy: "best-effort",
					AllowedUnsafeSysctls:  &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
					FailSwapOn:            &failSwapOn,
					ContainerLogMaxSizeMB: to.Int32Ptr(1000),
					ContainerLogMaxFiles:  to.Int32Ptr(99),
					PodMaxPids:            to.Int32Ptr(12345),
				}
			}, func(o *nodeBootstrappingOutput) {
				kubeletConfigFileContent, err := getBase64DecodedValue([]byte(o.vars["KUBELET_CONFIG_FILE_CONTENT"]))
				Expect(err).To(BeNil())
				Expect(kubeletConfigFileContent).To(ContainSubstring(`"serializeImagePulls": false`))
			}),
		Entry("AKSUbuntu2204 with SecurityProfile", "AKSUbuntu2204+SecurityProfile", "1.26.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ProxyAddress:            "https://test-pe-proxy",
						ContainerRegistryServer: "testserver.azurecr.io",
					},
				}
			}, nil),
		Entry("AKSUbuntu2204 IMDSRestriction with enable restriction and insert to mangle table", "AKSUbuntu2204+IMDSRestrictionOnWithMangleTable", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableIMDSRestriction = true
				config.InsertIMDSRestrictionRuleToMangleTable = true
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("true"))
				Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("true"))
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with enable restriction and not insert to mangle table", "AKSUbuntu2204+IMDSRestrictionOnWithFilterTable", "1.24.2",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.EnableIMDSRestriction = true
				config.InsertIMDSRestrictionRuleToMangleTable = false
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("true"))
				Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("false"))
			}),
		Entry("AKSUbuntu2204 IMDSRestriction with disable restriction", "AKSUbuntu2204+IMDSRestrictionOff", "1.24.2", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.EnableIMDSRestriction = false
		}, func(o *nodeBootstrappingOutput) {
			Expect(o.vars["ENABLE_IMDS_RESTRICTION"]).To(Equal("false"))
			Expect(o.vars["INSERT_IMDS_RESTRICTION_RULE_TO_MANGLE_TABLE"]).To(Equal("false"))
		}),
		Entry("AKSUbuntu2404 with custom osConfig for Ulimit", "AKSUbuntu2404+CustomLinuxOSConfigUlimit", ">=1.32.x",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = &datamodel.CustomLinuxOSConfig{
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: "75000",
						NoFile:          "1048",
					},
				}
				config.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSUbuntuContainerd2404
			}, func(o *nodeBootstrappingOutput) {
				Expect(o.vars["SHOULD_CONFIG_CONTAINERD_ULIMITS"]).To(Equal("true"))
				containerdUlimitContent := o.vars["CONTAINERD_ULIMITS"]
				Expect(containerdUlimitContent).NotTo(ContainSubstring("LimitNOFILE=1048"))
				Expect(containerdUlimitContent).To(ContainSubstring("LimitMEMLOCK=75000"))
			}),
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
	)
})

var _ = Describe("Assert generated customData and cseCmd for Windows", func() {
	DescribeTable("Generated customData and CSE", func(folder, k8sVersion string, configUpdator func(*datamodel.NodeBootstrappingConfiguration)) {
		cs := &datamodel.ContainerService{
			Location: "southcentralus",
			Type:     "Microsoft.ContainerService/ManagedClusters",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: k8sVersion,
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntime:     "docker",
						KubernetesImageBase:  "mcr.microsoft.com/oss/kubernetes/",
						WindowsContainerdURL: "https://k8swin.blob.core.windows.net/k8s-windows/containerd/containerplat-aks-test-0.0.8.zip",
						LoadBalancerSku:      "Standard",
						CustomHyperkubeImage: "mcr.microsoft.com/oss/kubernetes/hyperkube:v1.16.15-hotfix.20200903",
						ClusterSubnet:        "10.240.0.0/16",
						NetworkPlugin:        "azure",
						DockerBridgeSubnet:   "172.17.0.1/16",
						ServiceCIDR:          "10.0.0.0/16",
						EnableRbac:           to.BoolPtr(true),
						EnableSecureKubelet:  to.BoolPtr(true),
						UseInstanceMetadata:  to.BoolPtr(true),
						DNSServiceIP:         "10.0.0.10",
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					DNSPrefix:   "uttestdom",
					FQDN:        "uttestdom-dns-5d7c849e.hcp.southcentralus.azmk8s.io",
					Subnet:      "10.240.0.0/16",
					IPMasqAgent: true,
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{
					{
						Name:                "wpool2",
						VMSize:              "Standard_D2s_v3",
						StorageProfile:      "ManagedDisks",
						OSType:              datamodel.Windows,
						VnetSubnetID:        "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-36873793/subnet/aks-subnet",
						WindowsNameVersion:  "v2",
						AvailabilityProfile: datamodel.VirtualMachineScaleSets,
						CustomNodeLabels:    map[string]string{"kubernetes.azure.com/node-image-version": "AKSWindows-2019-17763.1577.201111"},
						Distro:              datamodel.Distro("aks-windows-2019"),
					},
				},
				LinuxProfile: &datamodel.LinuxProfile{
					AdminUsername: "azureuser",
				},
				WindowsProfile: &datamodel.WindowsProfile{
					ProvisioningScriptsPackageURL: "https://acs-mirror.azureedge.net/aks-engine/windows/provisioning/signedscripts-v0.0.4.zip",
					WindowsPauseImageURL:          "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.1",
					AdminUsername:                 "azureuser",
					AdminPassword:                 "replacepassword1234",
					WindowsPublisher:              "microsoft-aks",
					WindowsOffer:                  "aks-windows",
					ImageVersion:                  "17763.1577.201111",
					WindowsSku:                    "aks-2019-datacenter-core-smalldisk-2011",
				},
				ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{
					ClientID: "ClientID",
					Secret:   "Secret",
				},
				FeatureFlags: &datamodel.FeatureFlags{
					EnableWinDSR: false,
				},
			},
		}
		cs.Properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{
			KeyData: string("testsshkey"),
		}}

		// AKS always pass in te customHyperKubeImage to aks-e, so we don't really rely on
		// the default component version for "hyperkube", which is not set since 1.17
		if IsKubernetesVersionGe(k8sVersion, "1.17.0") {
			cs.Properties.OrchestratorProfile.KubernetesConfig.CustomHyperkubeImage = fmt.Sprintf("k8s.gcr.io/hyperkube-amd64:v%v", k8sVersion)
		}

		// WinDSR is only supported since 1.19
		if IsKubernetesVersionGe(k8sVersion, "1.19.0") {
			cs.Properties.FeatureFlags.EnableWinDSR = true
		}

		agentPool := cs.Properties.AgentPoolProfiles[0]

		k8sComponents := &datamodel.K8sComponents{}

		if IsKubernetesVersionGe(k8sVersion, "1.29.0") {
			// This is test only, credential provider version does not align with k8s version
			k8sComponents.WindowsCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz", k8sVersion, k8sVersion) //nolint:lll
			k8sComponents.LinuxCredentialProviderURL = fmt.Sprintf("https://acs-mirror.azureedge.net/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz", k8sVersion, k8sVersion)     //nolint:lll
		}

		kubeletConfig := map[string]string{
			"--address":                           "0.0.0.0",
			"--anonymous-auth":                    "false",
			"--authentication-token-webhook":      "true",
			"--authorization-mode":                "Webhook",
			"--cloud-config":                      "c:\\k\\azure.json",
			"--cgroups-per-qos":                   "false",
			"--client-ca-file":                    "c:\\k\\ca.crt",
			"--azure-container-registry-config":   "c:\\k\\azure.json",
			"--cloud-provider":                    "azure",
			"--cluster-dns":                       "10.0.0.10",
			"--cluster-domain":                    "cluster.local",
			"--enforce-node-allocatable":          "",
			"--event-qps":                         "0",
			"--eviction-hard":                     "",
			"--feature-gates":                     "RotateKubeletServerCertificate=true",
			"--hairpin-mode":                      "promiscuous-bridge",
			"--image-gc-high-threshold":           "85",
			"--image-gc-low-threshold":            "80",
			"--kube-reserved":                     "cpu=100m,memory=1843Mi",
			"--kubeconfig":                        "c:\\k\\config",
			"--max-pods":                          "30",
			"--network-plugin":                    "cni",
			"--node-status-update-frequency":      "10s",
			"--pod-infra-container-image":         "mcr.microsoft.com/oss/v2/kubernetes/pause:3.6",
			"--pod-max-pids":                      "-1",
			"--read-only-port":                    "0",
			"--resolv-conf":                       `""`,
			"--rotate-certificates":               "false",
			"--streaming-connection-idle-timeout": "4h",
			"--system-reserved":                   "memory=2Gi",
			"--tls-cipher-suites":                 "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_128_GCM_SHA256", //nolint:lll
		}

		config := &datamodel.NodeBootstrappingConfiguration{
			ContainerService:              cs,
			CloudSpecConfig:               datamodel.AzurePublicCloudSpecForTest,
			K8sComponents:                 k8sComponents,
			AgentPoolProfile:              agentPool,
			TenantID:                      "tenantID",
			SubscriptionID:                "subID",
			ResourceGroupName:             "resourceGroupName",
			UserAssignedIdentityClientID:  "userAssignedID",
			ConfigGPUDriverIfNeeded:       true,
			EnableGPUDevicePluginIfNeeded: false,
			EnableKubeletConfigFile:       false,
			EnableNvidia:                  false,
			KubeletConfig:                 kubeletConfig,
			PrimaryScaleSetName:           "akswpool2",
			SIGConfig: datamodel.SIGConfig{
				TenantID:       "tenantID",
				SubscriptionID: "subID",
				Galleries: map[string]datamodel.SIGGalleryConfig{
					"AKSUbuntu": {
						GalleryName:   "aksubuntu",
						ResourceGroup: "resourcegroup",
					},
					"AKSCBLMariner": {
						GalleryName:   "akscblmariner",
						ResourceGroup: "resourcegroup",
					},
					"AKSAzureLinux": {
						GalleryName:   "aksazurelinux",
						ResourceGroup: "resourcegroup",
					},
					"AKSWindows": {
						GalleryName:   "AKSWindows",
						ResourceGroup: "AKS-Windows",
					},
					"AKSUbuntuEdgeZone": {
						GalleryName:   "AKSUbuntuEdgeZone",
						ResourceGroup: "AKS-Ubuntu-EdgeZone",
					},
					"AKSFlatcar": {
						GalleryName:   "aksflatcar",
						ResourceGroup: "resourcegroup",
					},
				},
			},
		}

		if configUpdator != nil {
			configUpdator(config)
		}

		// customData
		ab, err := NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err := ab.GetNodeBootstrapping(context.Background(), config)
		Expect(err).To(BeNil())
		base64EncodedCustomData := nodeBootstrapping.CustomData
		customDataBytes, err := base64.StdEncoding.DecodeString(base64EncodedCustomData)
		customData := string(customDataBytes)
		Expect(err).To(BeNil())

		if generateTestData() {
			backfillCustomData(folder, customData)
		}

		expectedCustomData, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CustomData", folder))
		if err != nil {
			panic(err)
		}
		Expect(customData).To(Equal(string(expectedCustomData)))

		// CSE
		ab, err = NewAgentBaker()
		Expect(err).To(BeNil())
		nodeBootstrapping, err = ab.GetNodeBootstrapping(context.Background(), config)
		Expect(err).To(BeNil())
		cseCommand := nodeBootstrapping.CSE

		if generateTestData() {
			err = os.WriteFile(fmt.Sprintf("./testdata/%s/CSECommand", folder), []byte(cseCommand), 0644)
			Expect(err).To(BeNil())
		}

		expectedCSECommand, err := os.ReadFile(fmt.Sprintf("./testdata/%s/CSECommand", folder))
		if err != nil {
			panic(err)
		}
		Expect(cseCommand).To(Equal(string(expectedCSECommand)))

	}, Entry("AKSWindows2019 with k8s version 1.16", "AKSWindows2019+K8S116", "1.16.15", func(config *datamodel.NodeBootstrappingConfiguration) {
	}),
		Entry("AKSWindows2019 with k8s version 1.17", "AKSWindows2019+K8S117", "1.17.7", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.18", "AKSWindows2019+K8S118", "1.18.2", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.19", "AKSWindows2019+K8S119", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
		}),
		Entry("AKSWindows2019 with k8s version 1.19 + CSI", "AKSWindows2019+K8S119+CSI", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.CSIProxyURL = "https://acs-mirror.azureedge.net/csi-proxy/v0.1.0/binaries/csi-proxy.tar.gz"
			config.ContainerService.Properties.WindowsProfile.EnableCSIProxy = to.BoolPtr(true)
		}),
		Entry("AKSWindows2019 with CustomVnet", "AKSWindows2019+CustomVnet", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ClusterSubnet = "172.17.0.0/24"
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.ServiceCIDR = "172.17.255.0/24"
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetCidrs = []string{"172.17.0.0/16"}
			config.ContainerService.Properties.AgentPoolProfiles[0].VnetSubnetID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.Network/virtualNetworks/aks-vnet-07752737/subnet/subnet2" //nolint:lll
			config.KubeletConfig["--cluster-dns"] = "172.17.255.10"
		}),
		Entry("AKSWindows2019 with Managed Identity", "AKSWindows2019+ManagedIdentity", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{ClientID: "msi"}
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
			config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedID = "/subscriptions/359833f5/resourceGroups/MC_rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/k8s-agentpool" //nolint:lll
		}),
		Entry("AKSWindows2019 with custom cloud", "AKSWindows2019+CustomCloud", "1.19.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
				Name:                         "akscustom",
				McrURL:                       "mcr.microsoft.fakecustomcloud",
				RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
				ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
				PublishSettingsURL:           "",
				ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
				ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
				ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
				GalleryEndpoint:              "",
				KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
				GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
				ServiceBusEndpoint:           "",
				BatchManagementEndpoint:      "",
				StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
				SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
				TrafficManagerDNSSuffix:      "",
				KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
				ServiceBusEndpointSuffix:     "",
				ServiceManagementVMDNSSuffix: "",
				ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
				ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
				CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
				TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
				ResourceIdentifiers: datamodel.ResourceIdentifiers{
					Graph:               "",
					KeyVault:            "",
					Datalake:            "",
					Batch:               "",
					OperationalInsights: "",
					Storage:             "",
				},
			}
		}),
		Entry("AKSWindows2019 EnablePrivateClusterHostsConfigAgent", "AKSWindows2019+EnablePrivateClusterHostsConfigAgent", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				cs := config.ContainerService
				if cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster == nil {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &datamodel.PrivateCluster{EnableHostsConfigAgent: to.BoolPtr(true)}
				} else {
					cs.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.EnableHostsConfigAgent = to.BoolPtr(true)
				}
			}),
		Entry("AKSWindows2019 with kubelet client TLS bootstrapping enabled", "AKSWindows2019+KubeletClientTLSBootstrapping", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletClientTLSBootstrapToken = to.StringPtr("07401b.f395accd246ae52d")
			}),
		Entry("AKSWindows2019 with kubelet serving certificate rotation enabled", "AKSWindows2019+KubeletServingCertificateRotation", "1.29.7",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.KubeletConfig["--rotate-server-certificates"] = "true"
			}),
		Entry("AKSWindows2019 with k8s version 1.19 + FIPS", "AKSWindows2019+K8S119+FIPS", "1.19.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.FIPSEnabled = true
			}),
		Entry("AKSWindows2019 with SecurityProfile", "AKSWindows2019+SecurityProfile", "1.26.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:      true,
						ProxyAddress: "https://test-pe-proxy",
					},
				}
			}),
		Entry("AKSWindows2019 with out of tree credential provider", "AKSWindows2019+ootcredentialprovider", "1.29.0", func(config *datamodel.NodeBootstrappingConfiguration) {
			config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
			config.KubeletConfig["--image-credential-provider-config"] = "c:\\var\\lib\\kubelet\\credential-provider-config.yaml"
			config.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
		}),
		Entry("AKSWindows2019 with custom cloud and out of tree credential provider", "AKSWindows2019+CustomCloud+ootcredentialprovider", "1.29.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.WindowsProfile.AlwaysPullWindowsPauseImage = to.BoolPtr(true)
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
					Name:                         "akscustom",
					McrURL:                       "mcr.microsoft.fakecustomcloud",
					RepoDepotEndpoint:            "https://repodepot.azure.microsoft.fakecustomcloud/ubuntu",
					ManagementPortalURL:          "https://portal.azure.microsoft.fakecustomcloud/",
					PublishSettingsURL:           "",
					ServiceManagementEndpoint:    "https://management.core.microsoft.fakecustomcloud/",
					ResourceManagerEndpoint:      "https://management.azure.microsoft.fakecustomcloud/",
					ActiveDirectoryEndpoint:      "https://login.microsoftonline.microsoft.fakecustomcloud/",
					GalleryEndpoint:              "",
					KeyVaultEndpoint:             "https://vault.cloudapi.microsoft.fakecustomcloud/",
					GraphEndpoint:                "https://graph.cloudapi.microsoft.fakecustomcloud/",
					ServiceBusEndpoint:           "",
					BatchManagementEndpoint:      "",
					StorageEndpointSuffix:        "core.microsoft.fakecustomcloud",
					SQLDatabaseDNSSuffix:         "database.cloudapi.microsoft.fakecustomcloud",
					TrafficManagerDNSSuffix:      "",
					KeyVaultDNSSuffix:            "vault.cloudapi.microsoft.fakecustomcloud",
					ServiceBusEndpointSuffix:     "",
					ServiceManagementVMDNSSuffix: "",
					ResourceManagerVMDNSSuffix:   "cloudapp.azure.microsoft.fakecustomcloud/",
					ContainerRegistryDNSSuffix:   ".azurecr.microsoft.fakecustomcloud",
					CosmosDBDNSSuffix:            "documents.core.microsoft.fakecustomcloud/",
					TokenAudience:                "https://management.core.microsoft.fakecustomcloud/",
					ResourceIdentifiers: datamodel.ResourceIdentifiers{
						Graph:               "",
						KeyVault:            "",
						Datalake:            "",
						Batch:               "",
						OperationalInsights: "",
						Storage:             "",
					},
				}
				config.KubeletConfig["--image-credential-provider-config"] = "c:\\var\\lib\\kubelet\\credential-provider-config.yaml"
				config.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
			}),
		Entry("AKSWindows23H2Gen2 with NextGenNetworking", "AKSWindows23H2Gen2+NextGenNetworking", "1.29.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
					NextGenNetworkingEnabled: to.BoolPtr(true),
					NextGenNetworkingConfig:  to.StringPtr("{}"),
				}
			}),
		Entry("AKSWindows23H2Gen2 with NextGenNetworking enabled but no config", "AKSWindows23H2Gen2+NextGenNetworkingNoConfig", "1.29.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
					NextGenNetworkingEnabled: to.BoolPtr(true),
					NextGenNetworkingConfig:  nil,
				}
			}),
		Entry("AKSWindows23H2Gen2 with NextGenNetworking disabled", "AKSWindows23H2Gen2+NextGenNetworkingDisabled", "1.29.0",
			func(config *datamodel.NodeBootstrappingConfiguration) {
				config.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{
					NextGenNetworkingEnabled: to.BoolPtr(false),
					NextGenNetworkingConfig:  to.StringPtr("{}"),
				}
			}),
	)

})

func ignitionUnwrapEnvelope(ignitionFile []byte) []byte {
	// Unwrap the Ignition envelope
	var outer ign3_4.Config
	err := json.Unmarshal(ignitionFile, &outer)
	if err != nil {
		panic(err)
	}
	innerencoded := outer.Ignition.Config.Replace
	if innerencoded.Source == nil {
		panic("ignition missing replacement config")
	}
	inner, err := ignitionDecodeFileContents(innerencoded)
	if err != nil {
		panic(err)
	}
	return inner
}

func ignitionDecodeFileContents(input ign3_4.Resource) ([]byte, error) {
	// Decode data url format
	decodeddata, err := dataurl.DecodeString(*input.Source)
	if err != nil {
		return nil, err
	}
	contents := decodeddata.Data
	if input.Compression != nil && *input.Compression == "gzip" {
		contents, err = getGzipDecodedValue(contents)
		if err != nil {
			return nil, err
		}
	}
	return contents, nil
}

func writeInnerCustomData(outputname, customData string) error {
	ignitionInner := ignitionUnwrapEnvelope([]byte(customData))
	ignitionJson := json.RawMessage(ignitionInner)
	ignitionIndented, err := json.MarshalIndent(ignitionJson, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(outputname, ignitionIndented, 0644)
	return err
}

func backfillCustomData(folder, customData string) {
	if _, err := os.Stat(fmt.Sprintf("./testdata/%s", folder)); os.IsNotExist(err) {
		e := os.MkdirAll(fmt.Sprintf("./testdata/%s", folder), 0755)
		Expect(e).To(BeNil())
	}
	writeFileError := os.WriteFile(fmt.Sprintf("./testdata/%s/CustomData", folder), []byte(customData), 0644)
	Expect(writeFileError).To(BeNil())
	if strings.Contains(folder, "AKSWindows") {
		return
	}
	if strings.Contains(folder, "Flatcar") {
		err := writeInnerCustomData(fmt.Sprintf("testdata/%s/CustomData.inner", folder), customData)
		Expect(err).To(BeNil())
		return
	}
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

//lint:ignore U1000 this is used for test helpers in the future
func getGzipDecodedValue(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}

	output, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, fmt.Errorf("read from gzipped buffered string: %w", err)
	}

	return output, nil
}

func getBase64DecodedValue(data []byte) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func getDecodedFilesFromCustomdata(data []byte) (map[string]*decodedValue, error) {
	var customData cloudInit

	decodedCse, err := getGzipDecodedValue(data)
	if err != nil {
		decodedCse = data
	}

	if err := yaml.Unmarshal(decodedCse, &customData); err != nil {
		return nil, err
	}

	var files = make(map[string]*decodedValue)

	for _, val := range customData.WriteFiles {
		var encoding cseVariableEncoding
		maybeEncodedValue := val.Content

		if strings.Contains(val.Encoding, "gzip") {
			if maybeEncodedValue != "" {
				output, err := getGzipDecodedValue([]byte(maybeEncodedValue))
				if err != nil {
					return nil, fmt.Errorf("failed to decode gzip value: %q with error %w", maybeEncodedValue, err)
				}
				maybeEncodedValue = string(output)
				encoding = cseVariableEncodingGzip
			}
		}

		files[val.Path] = &decodedValue{
			value:    maybeEncodedValue,
			encoding: encoding,
		}
	}

	return files, nil
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
