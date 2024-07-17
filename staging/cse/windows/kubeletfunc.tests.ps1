BeforeAll {
    . $PSScriptRoot\..\..\..\parts\windows\windowscsehelper.ps1
    . $PSCommandPath.Replace('.tests.ps1','.ps1')
}

Describe 'Get-KubePackage' {
    BeforeEach {
        Mock Expand-Archive
        Mock Remove-Item
        Mock Logs-To-Event
        Mock DownloadFileOverHttp -MockWith {
            Param(
                $Url,
                $DestinationPath,
                $ExitCode
            )
            Write-Host "DownloadFileOverHttp -Url $Url -DestinationPath $DestinationPath -ExitCode $ExitCode"
        } -Verifiable

        $global:CacheDir = 'c:\akse-cache'
    }

    Context 'mapping file exists' {
        BeforeEach {
            Mock Test-Path -MockWith { $true }
            Mock Get-Content -MockWith {
                Param(
                    $Path
                )
@'
                {
                    "1.29.5":  "https://xxx.blob.core.windows.net/kubernetes/v1.29.5-hotfix.20240101/windowszip/v1.29.5-hotfix.20240101-1int.zip",
                    "1.29.2":  "https://xxx.blob.core.windows.net/kubernetes/v1.29.2-hotfix.20240101/windowszip/v1.29.2-hotfix.20240101-1int.zip",
                    "1.29.0":  "https://xxx.blob.core.windows.net/kubernetes/v1.29.0-hotfix.20240101/windowszip/v1.29.0-hotfix.20240101-1int.zip",
                    "1.28.3":  "https://xxx.blob.core.windows.net/kubernetes/v1.28.3-hotfix.20240101/windowszip/v1.28.3-hotfix.20240101-1int.zip",
                    "1.28.0":  "https://xxx.blob.core.windows.net/kubernetes/v1.28.0-hotfix.20240101/windowszip/v1.28.0-hotfix.20240101-1int.zip",
                    "1.27.1":  "https://xxx.blob.core.windows.net/kubernetes/v1.27.1-hotfix.20240101/windowszip/v1.27.1-hotfix.20240101-1int.zip",
                    "1.27.0":  "https://xxx.blob.core.windows.net/kubernetes/v1.27.0-hotfix.20240101/windowszip/v1.27.0-hotfix.20240101-1int.zip"
                }
'@
            }
        }

        It "KubeBinariesSASURL should be changed when the version exists in the mapping file" {
            $global:KubeBinariesVersion = '1.29.2'
            Get-KubePackage -KubeBinariesSASURL 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2/windowszip/v1.29.2-1int.zip'
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2-hotfix.20240101/windowszip/v1.29.2-hotfix.20240101-1int.zip' }
        }

        It "KubeBinariesSASURL should not be changed when the version does not exist in the mapping file" {
            $global:KubeBinariesVersion = '1.30.0'
            Get-KubePackage -KubeBinariesSASURL 'https://xxx.blob.core.windows.net/kubernetes/v1.30.0/windowszip/v1.30.0-1int.zip'
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq 'https://xxx.blob.core.windows.net/kubernetes/v1.30.0/windowszip/v1.30.0-1int.zip' }
        }
    }

    Context 'mapping file does not exist' {
        It "KubeBinariesSASURL should not be changed" {
            Mock Test-Path -MockWith { $false }
            $global:KubeBinariesVersion = '1.29.2'
            Get-KubePackage -KubeBinariesSASURL 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2/windowszip/v1.29.2-1int.zip'
            Assert-MockCalled -CommandName 'DownloadFileOverHttp' -Exactly -Times 1 -ParameterFilter { $Url -eq 'https://xxx.blob.core.windows.net/kubernetes/v1.29.2/windowszip/v1.29.2-1int.zip' }
        }
    }
}