# This csecmd is not used until servicePrincipalClientSecret is encoded,
# keeping this file for now to further facilicate the future removing ARM Template
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
-AgentKey {{ GetParameter "clientPrivateKey" }} 
-AADClientId {{ GetParameter "servicePrincipalClientId" }} 
-AADClientSecret ''{{ GetParameter "servicePrincipalClientSecret" }}''
-NetworkAPIVersion 2018-08-01;
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin'; 
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
Copy-Item $inputFile $outputFile;
Invoke-Expression('{0} {1}' -f $outputFile, $arguments);
\" > %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log 2>&1; exit $LASTEXITCODE 

