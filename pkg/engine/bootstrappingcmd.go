// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/aks-engine/pkg/api"
)

func getVMSSAgentBootstrappingCSE(cs *api.ContainerService, profile *api.AgentPoolProfile) compute.VirtualMachineScaleSetExtension {
	outBoundCmd := ""
	registry := ""
	ncBinary := "nc"
	if profile.IsCoreOS() {
		ncBinary = "ncat"
	}
	featureFlags := cs.Properties.FeatureFlags

	if !featureFlags.IsFeatureEnabled("BlockOutboundInternet") && cs.Properties.IsHostedMasterProfile() {
		if cs.GetCloudSpecConfig().CloudName == api.AzureChinaCloud {
			registry = `gcr.azk8s.cn 443`
		} else {
			registry = `aksrepos.azurecr.io 443`
		}
		outBoundCmd = `retrycmd_if_failure() { r=$1; w=$2; t=$3; shift && shift && shift; for i in $(seq 1 $r); do timeout $t ${@}; [ $? -eq 0  ] && break || if [ $i -eq $r ]; then return 1; else sleep $w; fi; done }; ERR_OUTBOUND_CONN_FAIL=50; retrycmd_if_failure 50 1 3 ` + ncBinary + ` -vz ` + registry + ` || exit $ERR_OUTBOUND_CONN_FAIL;`
	}

	var vmssCSE compute.VirtualMachineScaleSetExtension

	if profile.IsWindows() {
		return compute.VirtualMachineScaleSetExtension{
			Name: to.StringPtr("vmssCSE"),
			VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
				Publisher:               to.StringPtr("Microsoft.Compute"),
				Type:                    to.StringPtr("CustomScriptExtension"),
				TypeHandlerVersion:      to.StringPtr("1.8"),
				AutoUpgradeMinorVersion: to.BoolPtr(true),
				Settings:                map[string]interface{}{},
				ProtectedSettings: map[string]interface{}{
					"commandToExecute": "[concat('echo %DATE%,%TIME%,%COMPUTERNAME% && powershell.exe -ExecutionPolicy Unrestricted -command \"', '$arguments = ', variables('singleQuote'),'-MasterIP ',parameters('kubernetesEndpoint'),' -KubeDnsServiceIp ',parameters('kubeDnsServiceIp'),' -MasterFQDNPrefix ',variables('masterFqdnPrefix'),' -Location ',variables('location'),' -TargetEnvironment ',parameters('targetEnvironment'),' -AgentKey ',parameters('clientPrivateKey'),' -AADClientId ',variables('servicePrincipalClientId'),' -AADClientSecret ',variables('singleQuote'),variables('singleQuote'),base64(variables('servicePrincipalClientSecret')),variables('singleQuote'),variables('singleQuote'),' -NetworkAPIVersion ',variables('apiVersionNetwork'),' ',variables('singleQuote'), ' ; ', variables('windowsCustomScriptSuffix'), '\" > %SYSTEMDRIVE%\\AzureData\\CustomDataSetupScript.log 2>&1 ; exit $LASTEXITCODE')]",
				},
			},
		}
	} else {
		runInBackground := ""
		if featureFlags.IsFeatureEnabled("CSERunInBackground") {
			runInBackground = " &"
		}
		var userAssignedIDEnabled bool
		if cs.Properties.OrchestratorProfile != nil && cs.Properties.OrchestratorProfile.KubernetesConfig != nil {
			userAssignedIDEnabled = cs.Properties.OrchestratorProfile.KubernetesConfig.UserAssignedIDEnabled()
		} else {
			userAssignedIDEnabled = false
		}
		nVidiaEnabled := strconv.FormatBool(IsNvidiaEnabledSKU(profile.VMSize))
		sgxEnabled := strconv.FormatBool(IsSgxEnabledSKU(profile.VMSize))
		auditDEnabled := strconv.FormatBool(to.Bool(profile.AuditDEnabled))
		isVHD := strconv.FormatBool(profile.IsVHDDistro())

		commandExec := fmt.Sprintf("[concat('echo $(date),$(hostname); %s for i in $(seq 1 1200); do grep -Fq \"EOF\" /opt/azure/containers/provision.sh && break; if [ $i -eq 1200 ]; then exit 100; else sleep 1; fi; done; ', variables('provisionScriptParametersCommon'),%s,' IS_VHD=%s GPU_NODE=%s SGX_NODE=%s AUDITD_ENABLED=%s /usr/bin/nohup /bin/bash -c \"/bin/bash /opt/azure/containers/provision.sh >> /var/log/azure/cluster-provision.log 2>&1%s\"')]",
			outBoundCmd,
			generateUserAssignedIdentityClientIDParameter(userAssignedIDEnabled),
			isVHD,
			nVidiaEnabled,
			sgxEnabled,
			auditDEnabled,
			runInBackground)

		return compute.VirtualMachineScaleSetExtension{
			Name: to.StringPtr("vmssCSE"),
			VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
				Publisher:               to.StringPtr("Microsoft.Azure.Extensions"),
				Type:                    to.StringPtr("CustomScript"),
				TypeHandlerVersion:      to.StringPtr("2.0"),
				AutoUpgradeMinorVersion: to.BoolPtr(true),
				Settings:                map[string]interface{}{},
				ProtectedSettings: map[string]interface{}{
					"commandToExecute": commandExec,
				},
			},
		}
	}
}

func generateUserAssignedIdentityClientIDParameter(isUserAssignedIdentity bool) string {
	if isUserAssignedIdentity {
		return "' USER_ASSIGNED_IDENTITY_ID=',reference(concat('Microsoft.ManagedIdentity/userAssignedIdentities/', variables('userAssignedID')), '2018-11-30').clientId, ' '"
	}
	return "' USER_ASSIGNED_IDENTITY_ID=',' '"
}
