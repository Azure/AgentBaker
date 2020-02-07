// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"github.com/Azure/aks-engine/pkg/api"
)

func getVariables(cs *api.ContainerService, generatorCode string, aksEngineVersion string) paramsMap {
	return map[string]interface{}{
		"provisionScript":           getBase64EncodedGzippedCustomScript(kubernetesCSEMainScript, cs),
		"provisionSource":           getBase64EncodedGzippedCustomScript(kubernetesCSEHelpersScript, cs),
		"provisionInstalls":         getBase64EncodedGzippedCustomScript(kubernetesCSEInstall, cs),
		"provisionConfigs":          getBase64EncodedGzippedCustomScript(kubernetesCSEConfig, cs),
		"customSearchDomainsScript": getBase64EncodedGzippedCustomScript(kubernetesCustomSearchDomainsScript, cs),
		"dhcpv6SystemdService":      getBase64EncodedGzippedCustomScript(dhcpv6SystemdService, cs),
		"dhcpv6ConfigurationScript": getBase64EncodedGzippedCustomScript(dhcpv6ConfigurationScript, cs),
		"kubeletSystemdService":     getBase64EncodedGzippedCustomScript(kubeletSystemdService, cs),
		"systemdBPFMount":           getBase64EncodedGzippedCustomScript(systemdBPFMount, cs),
	}
}
