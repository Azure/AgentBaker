// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"fmt"
	"strconv"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/go-autorest/autorest/to"
)

func getBootstrappingCSE(cs *api.ContainerService, profile *api.AgentPoolProfile) string {
	if profile.IsWindows() {
		return "[concat('echo %DATE%,%TIME%,%COMPUTERNAME% && powershell.exe -ExecutionPolicy Unrestricted -command \"', '$arguments = ', variables('singleQuote'),'-MasterIP ',parameters('kubernetesEndpoint'),' -KubeDnsServiceIp ',parameters('kubeDnsServiceIp'),' -MasterFQDNPrefix ',variables('masterFqdnPrefix'),' -Location ',variables('location'),' -TargetEnvironment ',parameters('targetEnvironment'),' -AgentKey ',parameters('clientPrivateKey'),' -AADClientId ',variables('servicePrincipalClientId'),' -AADClientSecret ',variables('singleQuote'),variables('singleQuote'),base64(variables('servicePrincipalClientSecret')),variables('singleQuote'),variables('singleQuote'),' -NetworkAPIVersion ',variables('apiVersionNetwork'),' ',variables('singleQuote'), ' ; ', variables('windowsCustomScriptSuffix'), '\" > %SYSTEMDRIVE%\\AzureData\\CustomDataSetupScript.log 2>&1 ; exit $LASTEXITCODE')]"
	} else {

		runInBackground := ""
		nVidiaEnabled := strconv.FormatBool(IsNvidiaEnabledSKU(profile.VMSize))
		sgxEnabled := strconv.FormatBool(IsSgxEnabledSKU(profile.VMSize))
		auditDEnabled := strconv.FormatBool(to.Bool(profile.AuditDEnabled))
		isVHD := strconv.FormatBool(profile.IsVHDDistro())

		return fmt.Sprintf("[concat('echo $(date),$(hostname); %s for i in $(seq 1 1200); do grep -Fq \"EOF\" /opt/azure/containers/provision.sh && break; if [ $i -eq 1200 ]; then exit 100; else sleep 1; fi; done; ', variables('provisionScriptParametersCommon'),%s,' IS_VHD=%s GPU_NODE=%s SGX_NODE=%s AUDITD_ENABLED=%s /usr/bin/nohup /bin/bash -c \"/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1%s\"')]",
			outBoundCmd,
			generateUserAssignedIdentityClientIDParameter(cs),
			isVHD,
			nVidiaEnabled,
			sgxEnabled,
			auditDEnabled,
			runInBackground)
	}
}

func generateUserAssignedIdentityClientIDParameter(cs *api.ContainerService) string {
	if cs.Properties.OrchestratorProfile != nil &&
		cs.Properties.OrchestratorProfile.KubernetesConfig != nil &&
		cs.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedIDEnabled() {
		return "' USER_ASSIGNED_IDENTITY_ID=',reference(concat('Microsoft.ManagedIdentity/userAssignedIdentities/', variables('userAssignedID')), '2018-11-30').clientId, ' '"
	}
	return "' USER_ASSIGNED_IDENTITY_ID=',' '"
}
