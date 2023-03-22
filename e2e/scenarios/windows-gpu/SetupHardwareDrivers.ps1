####################################################################################################
#
# When running in a container in process isolation mode, the container itself needs the driver files
# for any hardware devices. These driver files needs to be exact copies of those in the host. Because
# the drivers on the host could be updated at any time, the preferred way to do this is when the
# container has just started, but before any apps that may need the device start. To facilitiate this, 
# Windows Container infrastructure automatically mounts the DriverStore from the underlying host,
# into the directory C:\Windows\System32\HostDriverStore. At startup time, The
# relevant driver files can be copied into c:\Windows\System32 in the container.
#
# This script is setup so that further devices can be added in the future. It is also written such
# that if a driver is not installed in the host, then it will just ignore that device type and 
# continue, not raising an error. The expectation is that the app will cope or take appropriate
# action if the device is missing.
#
####################################################################################################

# Funtion: Install-NvidiaGpuDriverFiles
#
# Function to setup the Nvidia CUDA video driver files given the hosts driver store directory.
# For video transcoding, the following 3 files + mapping are required (note that in some cases
# the file name in the destination changes):
#
# - nvcuda_loader64.dll     --> c:\windows\system32\nvcuda.dll
# - nvcuvid64.dll           --> c:\windows\system32\nvcuvid.dll
# - nvEncodeAPI64.dll       --> c:\windows\system32\nvEncode64.dll
# - nvml.dll                --> c:\windows\system32\nvml.dll
#
function Install-NvidiaGpuDriverFiles {
    param (
        $MountedHostDriverRepositoryPath
    )

	Write-Output "Setting up Nvidia GPU device drivers in container."

	# Nvidia: Check to see if there are Nvidia CUDA driver files. If installed on the host,
	# these files are stored in a directory that starts with 'nv_dispi.*'. Since there may
    # be re-installs (newer version etc), we choose the newest of these directories.
	$nvidiaDriverDirectories = Get-ChildItem -Filter nvgrid* -Path $MountedHostDriverRepositoryPath | Sort-Object LastWriteTime -Descending | Select-Object -first 1
	if ($nvidiaDriverDirectories.Count -lt 1)
	{
		Write-Output "No Nvidia drivers found. Skipping Nvidia GPU."
		return
	}

	$nvidiaDriverDirectoryFullPath = Join-Path -Path $driverFileRepositoryPath -ChildPath $nvidiaDriverDirectories[0]
	$nvidiaDriverDirectoryFullPathCopy = Join-Path -Path $nvidiaDriverDirectoryFullPath -ChildPath *

	#Copy-Item -Path $nvidiaDriverDirectoryFullPathCopy -Destination c:/windows/system32 -Recurse 

	$targetedFiles = @{ "nvCudaLoader64Dll" = "nvcuda_loader64.dll"; "nvCuvid64Dll" = "nvcuvid64.dll"; "nvEncodeAPI64Dll" = "nvEncodeAPI64.dll"; "nvmlDll" = "nvml.dll"; "nvSmiExe" = "nvidia-smi.exe"}

	$nvCudaLoader64DllFullPath	= Join-Path -Path $nvidiaDriverDirectoryFullPath	-ChildPath $targetedFiles.nvCudaLoader64Dll
	$nvCuvid64DllFullPath		= Join-Path -Path $nvidiaDriverDirectoryFullPath	-ChildPath $targetedFiles.nvCuvid64Dll
	$nvEncodeAPI64DllFullPath	= Join-Path -Path $nvidiaDriverDirectoryFullPath	-ChildPath $targetedFiles.nvEncodeAPI64Dll
	$nvmlDllFullPath			= Join-Path -Path $nvidiaDriverDirectoryFullPath	-ChildPath $targetedFiles.nvmlDll
	$nvSmiExeFullPath			= Join-Path -Path $nvidiaDriverDirectoryFullPath	-ChildPath $targetedFiles.nvSmiExe
	if (-not (Test-Path $nvCudaLoader64DllFullPath))
	{
		Write-Output "Nvidia driver file $nvCudaLoader64DllFullPath not found. Skipping Nvidia GPU."
		return
	}

	if (-not (Test-Path $nvCuvid64DllFullPath))
	{
		Write-Output "Nvidia driver file $nvCuvid64DllFullPath not found. Skipping Nvidia GPU."
		return
	}

	if (-not (Test-Path $nvEncodeAPI64DllFullPath))
	{
		Write-Output "Nvidia driver file $nvEncodeAPI64DllFullPath not found. Skipping Nvidia GPU."
		return
	}
	
	if (-not (Test-Path $nvmlDllFullPath))
	{
		Write-Output "Nvidia driver file $nvmlDllFullPath not found. Skipping Nvidia GPU."
		return
	}

    #Note: some of the files are copied to a different filename.
    $destinationNvCuda = Join-Path -Path c:/windows/system32 -ChildPath 'nvcuda.dll'
    Write-Output "Copying $nvCudaLoader64DllFullPath to $destinationNvCuda"
	Copy-Item -Path $nvCudaLoader64DllFullPath	-Destination $destinationNvCuda

	$destinationNvCuvid = Join-Path -Path c:/windows/system32 -ChildPath 'nvcuvid.dll'
    Write-Output "Copying $nvCuvid64DllFullPath to $destinationNvCuvid"
	Copy-Item -Path $nvCuvid64DllFullPath 		-Destination $destinationNvCuvid

	$destinationNvEncodeAPI64 = Join-Path -Path c:/windows/system32 -ChildPath "nvEncodeAPI64Dll"
    Write-Output "Copying $nvEncodeAPI64DllFullPath to $destinationNvEncodeAPI64"
	Copy-Item -Path $nvEncodeAPI64DllFullPath	-Destination $destinationNvEncodeAPI64
	
	$destinationNvml = Join-Path -Path c:/windows/system32 -ChildPath 'nvml.dll'
    Write-Output "Copying $nvmlDllFullPath to $destinationNvml"
	Copy-Item -Path $nvmlDllFullPath			-Destination $destinationNvml

	$destinationNvSmi = Join-Path -Path c:/windows/system32 -ChildPath 'nvidia-smi.exe'
    Write-Output "Copying $nvSmiExeFullPath to $destinationNvSmi"
	Copy-Item -Path $nvSmiExeFullPath 		-Destination $destinationNvSmi
	$driverfolder = Join-Path -Path $nvidiaDriverDirectoryFullPath -ChildPath *
	Copy-Item -Path $driverfolder -Destination c:/windows/system32

	Write-Output "Completed set up of Nvidia GPU device driver files."
}


# Main code
#
# Attempts to find the mounted host driver store from the underlying host and then call the Function
# for each of the devices needing setup. If the mounted host driver store cannot be found, then It
# is most likely that this script is not being run in the context of a container, so it just exits.
#
Write-Output "Setting up hardware device drivers in container."

$driverFileRepositoryPath = "c:\windows\system32\HostDriverStore\FileRepository"
$hostDriverStoreExists = Test-Path $driverFileRepositoryPath
if (-not $hostDriverStoreExists)
{
	Write-Output "The expected mounted 'host driver repository' path could not be found ($driverFileRepositoryPath). No driver files copied (OK if not running in container)."
	Exit
}

# Call each of the functions that attempts to setup/install different hardware driver files
Install-NvidiaGpuDriverFiles -MountedHostDriverRepositoryPath $driverFileRepositoryPath

Write-Output "Completed set up of hardware device drivers in container."