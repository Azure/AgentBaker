function Get-CalicoPackage {
    param(
        [parameter(Mandatory=$true)] $RootDir
    )

    Write-Log "Getting Calico package"
    DownloadFileOverHttp -Url $global:WindowsCalicoPackageURL -DestinationPath 'c:\calicowindows.zip' -ExitCode $global:WINDOWS_CSE_ERROR_DOWNLOAD_CALICO_PACKAGE
    Expand-Archive -Path 'c:\calicowindows.zip' -DestinationPath $RootDir -Force
    Remove-Item -Path 'c:\calicowindows.zip' -Force
}

function Set-CalicoStaticRules {
    param(
        [parameter(Mandatory=$true)] $CalicoRootDir
    )
    $fileName  = [Io.path]::Combine("$CalicoRootDir", "static-rules.json")
    echo '{
    "Provider": "AKS",
    "Rules": [
        {
            "Name": "EndpointPolicy",
            "Rule": {
                "Action": "Block",
                "Direction": "Out",
                "Id": "block-wireserver",
                "Priority": 200,
                "Protocol": 6,
                "RemoteAddresses": "168.63.129.16/32",
                "RemotePorts": "80",
                "RuleType": "Switch",
                "Type": "ACL"
            }
        },
        {
            "Name": "EndpointPolicy",
            "Rule": {
                "Action": "Block",
                "Direction": "Out",
                "Id": "block-wireserver-32526",
                "Priority": 200,
                "Protocol": 6,
                "RemoteAddresses": "168.63.129.16/32",
                "RemotePorts": "32526",
                "RuleType": "Switch",
                "Type": "ACL"
            }
        }
    ],
    "version": "0.1.0"
}' | Out-File -encoding ASCII -filepath $fileName
}

function SetConfigParameters {
    param(
        [parameter(Mandatory=$true)] $RootDir,
        [parameter(Mandatory=$true)] $OldString,
        [parameter(Mandatory=$true)] $NewString
    )

    (Get-Content $RootDir\config.ps1).replace($OldString, $NewString) | Set-Content $RootDir\config.ps1 -Force
}

function GetCalicoKubeConfig {
    param(
        [parameter(Mandatory=$true)] $RootDir,
        [parameter(Mandatory=$true)] $CalicoNamespace,
        [parameter(Mandatory=$false)] $SecretName = "calico-node",
        [parameter(Mandatory=$false)] $KubeConfigPath = "c:\\k\\config"
    )

    # When creating Windows agent pools with the system Linux agent pool, the service account for calico may not be available in provisioning Windows agent nodes.
    # So we need to wait here until the service account for calico is available
    $name=""
    $retryCount=0
    $retryInterval=5
    $maxRetryCount=120 # 10 minutes

    do {
        try {
            Write-Log "Retry $retryCount : Trying to get service account $SecretName"
            $name=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret -n $CalicoNamespace --field-selector=type=kubernetes.io/service-account-token --no-headers -o custom-columns=":metadata.name" | findstr $SecretName | select -first 1
            if (![string]::IsNullOrEmpty($name)) {
                break
            }
        } catch {
            Write-Log "Retry $retryCount : Failed to get service account $SecretName. Error: $_"
        }
        $retryCount++
        Write-Log "Retry $retryCount : Sleep $retryInterval and then retry to get service account $SecretName"
        Sleep $retryInterval
    } while ($retryCount -lt $maxRetryCount)

    if ([string]::IsNullOrEmpty($name)) {
        Set-ExitCode -ExitCode $global:WINDOWS_CSE_ERROR_CALICO_SERVICE_ACCOUNT_NOT_EXIST -ErrorMessage "$SecretName service account does not exist."
    }

    $ca=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret/$name -o jsonpath='{.data.ca\.crt}' -n $CalicoNamespace
    $tokenBase64=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret/$name -o jsonpath='{.data.token}' -n $CalicoNamespace
    $token=[System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String($tokenBase64))

    $server=findstr https:// $KubeConfigPath

    (Get-Content $RootDir\calico-kube-config.template).replace('<ca>', $ca).replace('<server>', $server.Trim()).replace('<token>', $token) | Set-Content $RootDir\calico-kube-config -Force
}

function Start-InstallCalico {
    param(
        [parameter(Mandatory=$true)] $RootDir,
        [parameter(Mandatory=$true)] $KubeServiceCIDR,
        [parameter(Mandatory=$true)] $KubeDnsServiceIp,
        [parameter(Mandatory=$false)] $CalicoNs = "calico-system"
    )
    Logs-To-Event -TaskName "AKS.WindowsCSE.InstallCalico" -TaskMessage "Start calico installation. WindowsCalicoPackageURL: $global:WindowsCalicoPackageURL"

    Write-Log "Download Calico"
    Get-CalicoPackage -RootDir $RootDir

    $CalicoDir  = [Io.path]::Combine("$RootDir", "CalicoWindows")

    Set-CalicoStaticRules -CalicoRootDir $CalicoDir

    SetConfigParameters -RootDir $CalicoDir -OldString "<your datastore type>" -NewString "kubernetes"
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd endpoints>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd key>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd cert>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd ca cert>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your service cidr>" -NewString $KubeServiceCIDR
    SetConfigParameters -RootDir $CalicoDir -OldString "<your dns server ips>" -NewString $KubeDnsServiceIp

    $calicoPackage=[IO.Path]::GetFileName($global:WindowsCalicoPackageURL)
    if ($calicoPackage -lt "calico-windows-v3.23.3.zip") {
        SetConfigParameters -RootDir $CalicoDir -OldString "CALICO_NETWORKING_BACKEND=`"vxlan`"" -NewString "CALICO_NETWORKING_BACKEND=`"none`""
        SetConfigParameters -RootDir $CalicoDir -OldString "KUBE_NETWORK = `"Calico.*`"" -NewString "KUBE_NETWORK = `"azure.*`""
    } else {
        SetConfigParameters -RootDir $CalicoDir -OldString "Set-EnvVarIfNotSet -var `"CALICO_NETWORKING_BACKEND`" -defaultValue `"vxlan`"" -NewString "Set-EnvVarIfNotSet -var `"CALICO_NETWORKING_BACKEND`" -defaultValue `"none`""
        SetConfigParameters -RootDir $CalicoDir -OldString "Set-EnvVarIfNotSet -var `"KUBE_NETWORK`" -defaultValue `"Calico.*`"" -NewString "Set-EnvVarIfNotSet -var `"KUBE_NETWORK`" -defaultValue `"azure.*`""
    }

    GetCalicoKubeConfig -RootDir $CalicoDir -CalicoNamespace $CalicoNs

    Write-Log "Install Calico"

    pushd $CalicoDir
    .\install-calico.ps1
    popd

    if ($calicoPackage -ge "calico-windows-v3.23.3.zip") {
        Write-Log "Starting Calico..."
        Write-Log "This may take several seconds if the vSwitch needs to be created."

        Start-Service CalicoNode
        Wait-ForCalicoInit
        Start-Service CalicoFelix

        while ((Get-Service | where Name -Like 'Calico*' | where Status -NE Running) -NE $null) {
            Write-Log "Waiting for the Calico services to be running..."
            Start-Sleep 1
        }

        Write-Log "Done, the Calico services are running:"
        Get-Service | where Name -Like 'Calico*'
    }
}
