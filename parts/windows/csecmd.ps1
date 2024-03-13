powershell.exe -ExecutionPolicy Unrestricted -command \"
$arguments = '
-MasterIP ''{{ GetKubernetesEndpoint }}''
-KubeDnsServiceIp ''{{ GetParameter "kubeDNSServiceIP" }}''
-MasterFQDNPrefix ''{{ GetParameter "masterEndpointDNSNamePrefix" }}''
-Location ''{{ GetVariable "location" }}''
{{if UserAssignedIDEnabled}}
-UserAssignedClientID ''{{ GetVariable "userAssignedIdentityID" }}''
{{ end }}
-TargetEnvironment ''{{ GetTargetEnvironment }}''
-AgentKey ''{{ GetParameter "clientPrivateKey" }}''
-AADClientId ''{{ GetParameter "servicePrincipalClientId" }}''
-AADClientSecret ''{{ GetParameter "encodedServicePrincipalClientSecret" }}''
-NetworkAPIVersion 2018-08-01
-LogFile %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log
-CSEResultFilePath %SYSTEMDRIVE%\AzureData\CSEResult.log';
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin';
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
if (!(Test-Path $inputFile)) { throw 'ExitCode: |49|, Output: |WINDOWS_CSE_ERROR_NO_CUSTOM_DATA_BIN|, Error: |C:\AzureData\CustomData.bin does not exist.|' };
Copy-Item $inputFile $outputFile -Force;
Invoke-Expression('{0} {1}' -f $outputFile, $arguments);
\"