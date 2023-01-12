powershell.exe -ExecutionPolicy Unrestricted -command \"
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
-AADClientSecret ''{{ GetParameter "encodedServicePrincipalClientSecret" }}''
-NetworkAPIVersion 2018-08-01
-LogFile %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log
-CSEResultFilePath %SYSTEMDRIVE%\AzureData\CSEResult.log';
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin';
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
if (!(Test-Path $inputFile)) { echo 49 | Out-File -FilePath '%SYSTEMDRIVE%\AzureData\CSEResult.log' -Encoding utf8; exit; };
Copy-Item $inputFile $outputFile;
Invoke-Expression('{0} {1}' -f $outputFile, $arguments);
\" >> %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log 2>&1; if (!(Test-Path %SYSTEMDRIVE%\AzureData\CSEResult.log)) { exit 50; }; $code=(Get-Content %SYSTEMDRIVE%\AzureData\CSEResult.log); exit $code