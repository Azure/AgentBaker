package agent

import (
	"crypto/x509"
	"encoding/base64"

	"encoding/pem"
	"errors"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// test certificate.
const encodedTestCert = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0lRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURWUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0eApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRUUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0="                     //nolint:lll
const testCertWithNewline = "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUgvVENDQmVXZ0F3SUJBZ0l\r\nRYUJZRTMvTTA4WEhZQ25OVm1jRkJjakFOQmdrcWhraUc5dzBCQVFzRkFEQnkKTVFzd0NRWURW\r\nUVFHRXdKVlV6RU9NQXdHQTFVRUNBd0ZWR1Y0WVhNeEVEQU9CZ05WQkFjTUIwaHZkWE4wYjI0\r\neApFVEFQQmdOVkJBb01DRk5UVENCRGIzSndNUzR3TEFZRFZRU\r\nUREQ1ZUVTB3dVkyOXRJRVZXSUZOVFRDQkpiblJsCmNtMWxaR2xoZEdVZ1EwRWdVbE5CSUZJek1CNFhEVEl3TURRd01UQXdOVGd6TTFvWERUSXhNRGN4TmpBd05UZ3oKTTFvd2diMHhDekFKQmdOVkJBWVRBbFZUTVE0d0RBWURWUVFJREFWVVpYaGhjekVRTUE0R0ExVUVCd3dIU0c5MQpjM1J2YmpFUk1BOEdBMVVFQ2d3SVUxTk1JRU52Y25BeEZqQVVCZ05WQkFVVERVNVdNakF3T0RFMk1UUXlORE14CkZEQVNCZ05WQkFNTUMzZDNkeTV6YzJ3dVkyOXRNUjB3R3dZRFZRUVBEQlJRY21sMllYUmxJRTl5WjJGdWFYcGgKZEdsdmJqRVhNQlVHQ3lzR0FRUUJnamM4QWdFQ0RBWk9aWFpoWkdFeEV6QVJCZ3NyQmdFRUFZSTNQQUlCQXhNQwpWVk13Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRREhoZVJrYmIxRkNjN3hSS3N0CndLMEpJR2FLWTh0N0piUzJiUTJiNllJSkRnbkh1SVlIcUJyQ1VWNzlvZWxpa2tva1JrRnZjdnBhS2luRkhEUUgKVXBXRUk2UlVFUlltU0NnM084V2k0MnVPY1YyQjVaYWJtWENrd2R4WTVFY2w1MUJiTThVbkdkb0FHYmRObWlSbQpTbVRqY3MrbGhNeGc0ZkZZNmxCcGlFVkZpR1VqR1JSKzYxUjY3THo2VTRLSmVMTmNDbTA3UXdGWUtCbXBpMDhnCmR5Z1N2UmRVdzU1Sm9wcmVkaitWR3RqVWtCNGhGVDRHUVgvZ2h0NjlSbHF6Lys4dTBkRVFraHVVdXVjcnFhbG0KU0d5NDNIUndCZkRLRndZZVdNN0NQTWQ1ZS9kTyt0MDh0OFBianpWVFR2NWhRRENzRVlJVjJUN0FGSTlTY054TQpraDcvQWdNQkFBR2pnZ05CTUlJRFBUQWZCZ05WSFNNRUdEQVdnQlMvd1ZxSC95ajZRVDM5dDAva0hhK2dZVmdwCnZUQi9CZ2dyQmdFRkJRY0JBUVJ6TUhFd1RRWUlLd1lCQlFVSE1BS0dRV2gwZEhBNkx5OTNkM2N1YzNOc0xtTnYKYlM5eVpYQnZjMmwwYjNKNUwxTlRUR052YlMxVGRXSkRRUzFGVmkxVFUwd3RVbE5CTFRRd09UWXRVak11WTNKMApNQ0FHQ0NzR0FRVUZCekFCaGhSb2RIUndPaTh2YjJOemNITXVjM05zTG1OdmJUQWZCZ05WSFJFRUdEQVdnZ3QzCmQzY3VjM05zTG1OdmJZSUhjM05zTG1OdmJUQmZCZ05WSFNBRVdEQldNQWNHQldlQkRBRUJNQTBHQ3lxRWFBR0cKOW5jQ0JRRUJNRHdHRENzR0FRUUJncWt3QVFNQkJEQXNNQ29HQ0NzR0FRVUZCd0lCRmg1b2RIUndjem92TDNkMwpkeTV6YzJ3dVkyOXRMM0psY0c5emFYUnZjbmt3SFFZRFZSMGxCQll3RkFZSUt3WUJCUVVIQXdJR0NDc0dBUVVGCkJ3TUJNRWdHQTFVZEh3UkJNRDh3UGFBN29EbUdOMmgwZEhBNkx5OWpjbXh6TG5OemJDNWpiMjB2VTFOTVkyOXQKTFZOMVlrTkJMVVZXTFZOVFRDMVNVMEV0TkRBNU5pMVNNeTVqY213d0hRWURWUjBPQkJZRUZBREFGVUlhenc1cgpaSUhhcG5SeElVbnB3K0dMTUE0R0ExVWREd0VCL3dRRUF3SUZvRENDQVgwR0Npc0dBUVFCMW5rQ0JBSUVnZ0Z0CkJJSUJhUUZuQUhjQTlseVVMOUYzTUNJVVZCZ0lNSlJXanVOTkV4a3p2OThNTHlBTHpFN3haT01BQUFGeE0waG8KYndBQUJBTUFTREJHQWlFQTZ4ZWxpTlI4R2svNjNwWWRuUy92T3gvQ2pwdEVNRXY4OVdXaDEvdXJXSUVDSVFEeQpCcmVIVTI1RHp3dWtRYVJRandXNjU1WkxrcUNueGJ4UVdSaU9lbWo5SkFCMUFKUWd2QjZPMVkxc2lITWZnb3NpCkxBM1IyazFlYkUrVVBXSGJUaTlZVGFMQ0FBQUJjVE5JYU53QUFBUURBRVl3UkFJZ0dSRTR3emFiTlJkRDhrcS8KdkZQM3RRZTJobTB4NW5YdWxvd2g0SWJ3M2xrQ0lGWWIvM2xTRHBsUzdBY1I0citYcFd0RUtTVEZXSm1OQ1JiYwpYSnVyMlJHQkFIVUE3c0NWN28xeVpBK1M0OE81RzhjU28ybHFDWHRMYWhvVU9PWkhzc3Z0eGZrQUFBRnhNMGhvCjh3QUFCQU1BUmpCRUFpQjZJdmJvV3NzM1I0SXRWd2plYmw3RDN5b0ZhWDBORGgyZFdoaGd3Q3hySHdJZ0NmcTcKb2NNQzV0KzFqaTVNNXhhTG1QQzRJK1dYM0kvQVJrV1N5aU83SVFjd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dJQgpBQ2V1dXI0UW51anFtZ3VTckhVM21oZitjSm9kelRRTnFvNHRkZStQRDEvZUZkWUFFTHU4eEYrMEF0N3hKaVBZCmk1Ukt3aWx5UDU2diszaVkyVDlsdzdTOFRKMDQxVkxoYUlLcDE0TXpTVXpSeWVvT0FzSjdRQURNQ2xIS1VEbEgKVVUycE51bzg4WTZpZ292VDNic253Sk5pRVFOcXltU1NZaGt0dzB0YWR1b3FqcVhuMDZnc1Zpb1dUVkRYeXNkNQpxRXg0dDZzSWdJY01tMjZZSDF2SnBDUUVoS3BjMnkwN2dSa2tsQlpSdE1qVGh2NGNYeXlNWDd1VGNkVDdBSkJQCnVlaWZDb1YyNUp4WHVvOGQ1MTM5Z3dQMUJBZTdJQlZQeDJ1N0tOL1V5T1hkWm13TWYvVG1GR3dEZENmc3lIZi8KWnNCMndMSG96VFlvQVZtUTlGb1UxSkxnY1ZpdnFKK3ZObEJoSFhobHhNZE4wajgwUjlOejZFSWdsUWplSzNPOApJL2NGR20vQjgrNDJoT2xDSWQ5WmR0bmRKY1JKVmppMHdEMHF3ZXZDYWZBOWpKbEh2L2pzRStJOVV6NmNwQ3loCnN3K2xyRmR4VWdxVTU4YXhxZUs4OUZSK05vNHEwSUlPK0ppMXJKS3I5bmtTQjBCcVhvelZuRTFZQi9LTHZkSXMKdVlaSnVxYjJwS2t1K3p6VDZnVXdIVVRadkJpTk90WEw0Tnh3Yy9LVDdXek9TZDJ3UDEwUUk4REtnNHZmaU5EcwpIV21CMWM0S2ppNmdPZ0E1dVNVemFHbXEvdjRWbmNLNVVyK245TGJmbmZMYzI4SjVmdC9Hb3Rpbk15RGszaWFyCkYxMFlscWNPbWVYMXVGbUtiZGkvWG9yR2xrQ29NRjNURHg4cm1wOURCaUIvCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=" //nolint:lll

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
				localDNSCorefile, err := GetGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
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
				localDNSCorefile, err := GetGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
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
				localDNSCorefile, err := GetGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
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
				localDNSCorefile, err := GetGzipDecodedValue([]byte(localDNSCoreFileGzippedBase64Decoded))
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
})

func verifyCertsEncoding(cert string) error {
	certPEM, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("pem decode block is nil")
	}

	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	return nil
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
