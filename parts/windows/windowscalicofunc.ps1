function Get-CalicoPackage {
    param(
        [parameter(Mandatory=$true)] $RootDir
    )

    Write-Log "Getting Calico package"
    DownloadFileOverHttp -Url $global:WindowsCalicoPackageURL -DestinationPath 'c:\calicowindows.zip'
    Expand-Archive -Path 'c:\calicowindows.zip' -DestinationPath $RootDir -Force
    Remove-Item -Path 'c:\calicowindows.zip' -Force
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

    $name=c:\k\kubectl.exe --kubeconfig=$KubeConfigPath get secret -n $CalicoNamespace --field-selector=type=kubernetes.io/service-account-token --no-headers -o custom-columns=":metadata.name" | findstr $SecretName | select -first 1
    if ([string]::IsNullOrEmpty($name)) {
        throw "$SecretName service account does not exist."
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

    Write-Log "Download Calico"
    Get-CalicoPackage -RootDir $RootDir

    $CalicoDir = $RootDir + "CalicoWindows"

    SetConfigParameters -RootDir $CalicoDir -OldString "<your datastore type>" -NewString "kubernetes"
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd endpoints>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd key>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd cert>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your etcd ca cert>" -NewString ""
    SetConfigParameters -RootDir $CalicoDir -OldString "<your service cidr>" -NewString $KubeServiceCIDR
    SetConfigParameters -RootDir $CalicoDir -OldString "<your dns server ips>" -NewString $KubeDnsServiceIp
    SetConfigParameters -RootDir $CalicoDir -OldString "CALICO_NETWORKING_BACKEND=`"vxlan`"" -NewString "CALICO_NETWORKING_BACKEND=`"none`""
    SetConfigParameters -RootDir $CalicoDir -OldString "KUBE_NETWORK = `"Calico.*`"" -NewString "KUBE_NETWORK = `"azure.*`""

    GetCalicoKubeConfig -RootDir $CalicoDir -CalicoNamespace $CalicoNs

    Write-Log "Install Calico"

    pushd $CalicoDir
    .\install-calico.ps1
    popd
}
