package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
)

// The LocalDNS Corefile template exists twice: in pkg/agent/baker.go, which renders the Corefile
// classic CSE nodes boot with, and in localdns.toml.gtpl, which renders it for scriptless nodes and
// for every live patch. If they drift, a live patch changes every classic CSE node's Corefile with no
// profile change behind it. So both render the same profiles to the same golden files:
// TestLocalDNSCorefileParity here, and TestLocalDNSCorefileParity in pkg/agent/localdns_parity_test.go
// with the same profiles in its own data model. Regenerate the golden files with
// GENERATE_TEST_DATA=true; pkg/agent's test then fails until baker.go renders the same.
const localDNSCorefileParityDir = "testdata/LocalDNSCorefileParity"

// Not the default CoreDNS service IP, so a side that loses it renders a different Corefile.
const localDNSCorefileParityCoreDNSServiceIP = "10.2.0.10"

func localDNSCorefileParityProfiles() map[string]*aksnodeconfigv1.LocalDnsProfile {
	return map[string]*aksnodeconfigv1.LocalDnsProfile{
		"NoOverrides": {EnableLocalDns: true},
		// The overrides the RP sends by default.
		"Defaults": {
			EnableLocalDns: true,
			VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
				".": {
					QueryLogging:                "Error",
					Protocol:                    "PreferUDP",
					ForwardDestination:          "VnetDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Ptr(int32(1000)),
					CacheDurationInSeconds:      to.Ptr(int32(3600)),
					ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
					ServeStale:                  "Verify",
				},
				"cluster.local": {
					QueryLogging:                "Error",
					Protocol:                    "ForceTCP",
					ForwardDestination:          "ClusterCoreDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Ptr(int32(1000)),
					CacheDurationInSeconds:      to.Ptr(int32(3600)),
					ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
					ServeStale:                  "Disable",
				},
			},
			KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
				".": {
					QueryLogging:                "Error",
					Protocol:                    "PreferUDP",
					ForwardDestination:          "ClusterCoreDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Ptr(int32(1000)),
					CacheDurationInSeconds:      to.Ptr(int32(3600)),
					ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
					ServeStale:                  "Verify",
				},
				"cluster.local": {
					QueryLogging:                "Error",
					Protocol:                    "ForceTCP",
					ForwardDestination:          "ClusterCoreDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Ptr(int32(1000)),
					CacheDurationInSeconds:      to.Ptr(int32(3600)),
					ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
					ServeStale:                  "Disable",
				},
			},
		},
		// Every branch of the template: each value of each override field, the hosts block (rendered
		// separately), health checks with and without no_rec and domain, and a serve stale policy
		// both where it applies (a root zone) and where it does not.
		"EveryBranch": {
			EnableLocalDns: true,
			VnetDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
				".": localDNSCorefileParityEveryField(),
				"contoso.internal": {
					QueryLogging:                  "Error",
					Protocol:                      "PreferUDP",
					ForwardDestination:            "ClusterCoreDNS",
					ForwardPolicy:                 "Random",
					MaxConcurrent:                 to.Ptr(int32(100)),
					CacheDurationInSeconds:        to.Ptr(int32(60)),
					ServeStaleDurationInSeconds:   to.Ptr(int32(30)),
					ServeStale:                    "Verify",
					FailfastAllUnhealthyUpstreams: to.Ptr(false),
					HealthCheck:                   &aksnodeconfigv1.LocalDnsHealthCheck{Duration: to.Ptr("1s")},
				},
				"fabrikam.internal": {
					ForwardDestination:          "VnetDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Ptr(int32(10)),
					CacheDurationInSeconds:      to.Ptr(int32(5)),
					ServeStaleDurationInSeconds: to.Ptr(int32(5)),
					ServeStale:                  "Disable",
					ServeStalePolicy:            "PreferPositive",
				},
			},
			KubeDnsOverrides: map[string]*aksnodeconfigv1.LocalDnsOverrides{
				".": {
					QueryLogging:                  "Error",
					Protocol:                      "PreferUDP",
					ForwardDestination:            "VnetDNS",
					ForwardPolicy:                 "Random",
					MaxConcurrent:                 to.Ptr(int32(2000)),
					CacheDurationInSeconds:        to.Ptr(int32(900)),
					ServeStaleDurationInSeconds:   to.Ptr(int32(300)),
					ServeStale:                    "Verify",
					FailfastAllUnhealthyUpstreams: to.Ptr(true),
					HealthCheck:                   &aksnodeconfigv1.LocalDnsHealthCheck{Duration: to.Ptr("2s"), Domain: to.Ptr("kube.contoso.com")},
					ServeStalePolicy:              "PreferPositive",
				},
				"cluster.local": {
					QueryLogging:                "Log",
					Protocol:                    "ForceTCP",
					ForwardDestination:          "ClusterCoreDNS",
					ForwardPolicy:               "RoundRobin",
					MaxConcurrent:               to.Ptr(int32(1000)),
					CacheDurationInSeconds:      to.Ptr(int32(3600)),
					ServeStaleDurationInSeconds: to.Ptr(int32(3600)),
					ServeStale:                  "Immediate",
				},
				"contoso.internal": {
					ForwardDestination:          "VnetDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Ptr(int32(10)),
					CacheDurationInSeconds:      to.Ptr(int32(5)),
					ServeStaleDurationInSeconds: to.Ptr(int32(5)),
					ServeStale:                  "Disable",
				},
			},
		},
	}
}

// localDNSCorefileParityEveryField sets every LocalDnsOverrides field to a non-default value.
func localDNSCorefileParityEveryField() *aksnodeconfigv1.LocalDnsOverrides {
	return &aksnodeconfigv1.LocalDnsOverrides{
		QueryLogging:                  "Log",
		Protocol:                      "ForceTCP",
		ForwardDestination:            "VnetDNS",
		ForwardPolicy:                 "RoundRobin",
		MaxConcurrent:                 to.Ptr(int32(500)),
		CacheDurationInSeconds:        to.Ptr(int32(1800)),
		ServeStaleDurationInSeconds:   to.Ptr(int32(600)),
		ServeStale:                    "Immediate",
		FailfastAllUnhealthyUpstreams: to.Ptr(true),
		HealthCheck: &aksnodeconfigv1.LocalDnsHealthCheck{
			Duration: to.Ptr("500ms"),
			NoRec:    to.Ptr(true),
			Domain:   to.Ptr("health.contoso.com"),
		},
		ServeStalePolicy: "PreferPositive",
	}
}

func TestLocalDNSCorefileParity(t *testing.T) {
	// A field added to LocalDnsOverrides fails here until it is set, so the golden files cover it.
	t.Run("EveryBranch sets every override field", func(t *testing.T) {
		requireEveryExportedFieldSet(t, reflect.ValueOf(localDNSCorefileParityEveryField()))
	})

	for name, profile := range localDNSCorefileParityProfiles() {
		for _, includeHostsPlugin := range []bool{false, true} {
			file := name
			if includeHostsPlugin {
				file += "+HostsPlugin"
			}
			t.Run(file, func(t *testing.T) {
				corefile, err := GenerateLocalDnsCorefileFromAKSNodeConfig(&aksnodeconfigv1.Configuration{
					ClusterConfig: &aksnodeconfigv1.ClusterConfig{
						ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
							CoreDnsServiceIp: localDNSCorefileParityCoreDNSServiceIP,
						},
					},
					LocalDnsProfile: profile,
				}, includeHostsPlugin)
				require.NoError(t, err)

				path := filepath.Join(localDNSCorefileParityDir, file+".corefile")
				if os.Getenv("GENERATE_TEST_DATA") == "true" {
					require.NoError(t, os.MkdirAll(localDNSCorefileParityDir, 0755))
					require.NoError(t, os.WriteFile(path, []byte(corefile), 0644))
				}
				want, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, string(want), corefile,
					"localdns.toml.gtpl no longer renders the golden file: regenerate it with GENERATE_TEST_DATA=true and make baker.go's template render the same")
			})
		}
	}
}

// requireEveryExportedFieldSet fails for each exported field of the struct v points to that holds
// its zero value, following pointers to structs.
func requireEveryExportedFieldSet(t *testing.T, v reflect.Value) {
	t.Helper()
	v = v.Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if !field.IsExported() {
			continue
		}
		value := v.Field(i)
		require.Falsef(t, value.IsZero(), "%s.%s is not set", v.Type().Name(), field.Name)
		if value.Kind() == reflect.Pointer && value.Elem().Kind() == reflect.Struct {
			requireEveryExportedFieldSet(t, value)
		}
	}
}
