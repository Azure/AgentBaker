package agent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

// The LocalDNS Corefile template exists twice: in baker.go, which renders the Corefile classic CSE
// nodes boot with, and in aks-node-controller's localdns.toml.gtpl, which renders it for scriptless
// nodes and for every live patch. If they drift, a live patch changes every classic CSE node's
// Corefile with no profile change behind it. So both render the same profiles to the same golden
// files: TestLocalDNSCorefileParity here, and TestLocalDNSCorefileParity in
// aks-node-controller/parser/localdns_parity_test.go, which owns the golden files and builds the same
// profiles in its own data model.
const localDNSCorefileParityDir = "../../aks-node-controller/parser/testdata/LocalDNSCorefileParity"

// Not the default CoreDNS service IP, so a side that loses it renders a different Corefile.
const localDNSCorefileParityCoreDNSServiceIP = "10.2.0.10"

func localDNSCorefileParityProfiles() map[string]*datamodel.LocalDNSProfile {
	return map[string]*datamodel.LocalDNSProfile{
		"NoOverrides": {EnableLocalDNS: true},
		// The overrides the RP sends by default.
		"Defaults": {
			EnableLocalDNS: true,
			VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
				".": {
					QueryLogging:                "Error",
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
					QueryLogging:                "Error",
					Protocol:                    "ForceTCP",
					ForwardDestination:          "ClusterCoreDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Int32Ptr(1000),
					CacheDurationInSeconds:      to.Int32Ptr(3600),
					ServeStaleDurationInSeconds: to.Int32Ptr(3600),
					ServeStale:                  "Disable",
				},
			},
		},
		// Every branch of the template: each value of each override field, the hosts block (rendered
		// separately), health checks with and without no_rec and domain, and a serve stale policy
		// both where it applies (a root zone) and where it does not.
		"EveryBranch": {
			EnableLocalDNS: true,
			VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
				".": localDNSCorefileParityEveryField(),
				"contoso.internal": {
					QueryLogging:                  "Error",
					Protocol:                      "PreferUDP",
					ForwardDestination:            "ClusterCoreDNS",
					ForwardPolicy:                 "Random",
					MaxConcurrent:                 to.Int32Ptr(100),
					CacheDurationInSeconds:        to.Int32Ptr(60),
					ServeStaleDurationInSeconds:   to.Int32Ptr(30),
					ServeStale:                    "Verify",
					FailfastAllUnhealthyUpstreams: to.BoolPtr(false),
					HealthCheck:                   &datamodel.LocalDNSHealthCheck{Duration: to.StringPtr("1s")},
				},
				"fabrikam.internal": {
					ForwardDestination:          "VnetDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Int32Ptr(10),
					CacheDurationInSeconds:      to.Int32Ptr(5),
					ServeStaleDurationInSeconds: to.Int32Ptr(5),
					ServeStale:                  "Disable",
					ServeStalePolicy:            "PreferPositive",
				},
			},
			KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{
				".": {
					QueryLogging:                  "Error",
					Protocol:                      "PreferUDP",
					ForwardDestination:            "VnetDNS",
					ForwardPolicy:                 "Random",
					MaxConcurrent:                 to.Int32Ptr(2000),
					CacheDurationInSeconds:        to.Int32Ptr(900),
					ServeStaleDurationInSeconds:   to.Int32Ptr(300),
					ServeStale:                    "Verify",
					FailfastAllUnhealthyUpstreams: to.BoolPtr(true),
					HealthCheck:                   &datamodel.LocalDNSHealthCheck{Duration: to.StringPtr("2s"), Domain: to.StringPtr("kube.contoso.com")},
					ServeStalePolicy:              "PreferPositive",
				},
				"cluster.local": {
					QueryLogging:                "Log",
					Protocol:                    "ForceTCP",
					ForwardDestination:          "ClusterCoreDNS",
					ForwardPolicy:               "RoundRobin",
					MaxConcurrent:               to.Int32Ptr(1000),
					CacheDurationInSeconds:      to.Int32Ptr(3600),
					ServeStaleDurationInSeconds: to.Int32Ptr(3600),
					ServeStale:                  "Immediate",
				},
				"contoso.internal": {
					ForwardDestination:          "VnetDNS",
					ForwardPolicy:               "Sequential",
					MaxConcurrent:               to.Int32Ptr(10),
					CacheDurationInSeconds:      to.Int32Ptr(5),
					ServeStaleDurationInSeconds: to.Int32Ptr(5),
					ServeStale:                  "Disable",
				},
			},
		},
	}
}

// localDNSCorefileParityEveryField sets every LocalDNSOverrides field to a non-default value.
func localDNSCorefileParityEveryField() *datamodel.LocalDNSOverrides {
	return &datamodel.LocalDNSOverrides{
		QueryLogging:                  "Log",
		Protocol:                      "ForceTCP",
		ForwardDestination:            "VnetDNS",
		ForwardPolicy:                 "RoundRobin",
		MaxConcurrent:                 to.Int32Ptr(500),
		CacheDurationInSeconds:        to.Int32Ptr(1800),
		ServeStaleDurationInSeconds:   to.Int32Ptr(600),
		ServeStale:                    "Immediate",
		FailfastAllUnhealthyUpstreams: to.BoolPtr(true),
		HealthCheck: &datamodel.LocalDNSHealthCheck{
			Duration: to.StringPtr("500ms"),
			NoRec:    to.BoolPtr(true),
			Domain:   to.StringPtr("health.contoso.com"),
		},
		ServeStalePolicy: "PreferPositive",
	}
}

func TestLocalDNSCorefileParity(t *testing.T) {
	// A field added to LocalDNSOverrides fails here until it is set, so the golden files cover it.
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
				config := &datamodel.NodeBootstrappingConfiguration{
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
						KubernetesConfig: &datamodel.KubernetesConfig{DNSServiceIP: localDNSCorefileParityCoreDNSServiceIP},
						LocalDNSProfile:  profile,
					},
				}
				corefile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, includeHostsPlugin)
				require.NoError(t, err)

				want, err := os.ReadFile(filepath.Join(localDNSCorefileParityDir, file+".corefile"))
				require.NoError(t, err)
				require.Equal(t, string(want), corefile,
					"baker.go's LocalDNS Corefile template no longer matches aks-node-controller's localdns.toml.gtpl")
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
