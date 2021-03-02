echo %DATE%,%TIME%,%COMPUTERNAME% && powershell.exe -ExecutionPolicy Unrestricted -command \"
$arguments = '
-MasterIP {{ GetKubernetesEndpoint }}
-KubeDnsServiceIp {{ GetParameter "kubeDNSServiceIP" }}
-MasterFQDNPrefix {{ GetParameter "masterEndpointDNSNamePrefix" }}
-Location {{ GetVariable "location" }}
{{if UserAssignedIDEnabled}}
-UserAssignedClientID {{ GetVariable "userAssignedIdentityID" }}
{{ end }}
-TargetEnvironment {{ GetTargetEnvironment }}
{{- if IsKubeletClientTLSBootstrappingEnabled}}
-TLSBootstrapToken {{ GetTLSBootstrapTokenForKubeConfig }}
{{- else }}
-AgentKey {{ GetParameter "clientPrivateKey" }}
{{- end }}
-AADClientId {{ GetParameter "servicePrincipalClientId" }}
-AADClientSecret ''{{ GetParameter "encodedServicePrincipalClientSecret" }}''
-NetworkAPIVersion 2018-08-01';
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin';
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
Copy-Item $inputFile $outputFile;
Invoke-Expression('{0} {1}' -f $outputFile, $arguments);
\" > %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log 2>&1; exit $LASTEXITCODE