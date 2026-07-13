function Get-RSAKeyContainerHandleLog {
  param(
    [Parameter(Mandatory = $true)]
    [string]$HandlePath,
    [Parameter(Mandatory = $true)]
    [string]$OutputPath
  )

  if (-not (Test-Path -LiteralPath $HandlePath -PathType Leaf)) {
    Write-Host "Skipping RSA key container handle collection because $HandlePath does not exist"
    return ""
  }

  Write-Host "Collecting RSA key container handles"
  & $HandlePath -accepteula rsa-key-container *> $OutputPath
  $handleExitCode = $LASTEXITCODE
  $outputFile = Get-Item -LiteralPath $OutputPath -ErrorAction SilentlyContinue

  if ($outputFile -and $outputFile.Length -gt 0) {
    if ($handleExitCode -ne 0) {
      Write-Host "handle64.exe exited with code $handleExitCode"
    }
    return $OutputPath
  }

  if ($outputFile) {
    Remove-Item -LiteralPath $OutputPath -Force
  }
  Write-Host "handle64.exe did not produce any output"

  if ($handleExitCode -ne 0) {
    Write-Host "handle64.exe exited with code $handleExitCode"
  }
  return ""
}

Export-ModuleMember -Function Get-RSAKeyContainerHandleLog
