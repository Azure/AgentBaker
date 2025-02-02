powershell.exe -ExecutionPolicy Unrestricted -command \"
$inputFile = '%SYSTEMDRIVE%\AzureData\CustomData.bin';
$outputFile = '%SYSTEMDRIVE%\AzureData\CustomDataSetupScript.ps1';
if (!(Test-Path $inputFile)) { throw 'ExitCode: |49|, Output: |WINDOWS_CSE_ERROR_NO_CUSTOM_DATA_BIN|, Error: |$inputFile does not exist.|' };
Copy-Item $inputFile $outputFile -Force;
PowerShell -File $outputFile -CSEResultFilePath %SYSTEMDRIVE%\AzureData\provision.complete >> %SYSTEMDRIVE%\AzureData\CustomDataSetupScript.log 2>&1;
if (!(Test-Path %SYSTEMDRIVE%\AzureData\provision.complete)) { throw 'ExitCode: |50|, Output: |WINDOWS_CSE_ERROR_NO_CSE_RESULT_LOG|, Error: |C:\AzureData\provision.complete is not generated.|'; }; $result=(Get-Content %SYSTEMDRIVE%\AzureData\provision.complete); if($result -ne '0') { throw $result; };
\"