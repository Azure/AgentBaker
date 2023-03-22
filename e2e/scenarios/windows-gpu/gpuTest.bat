
Powershell.exe ./SetupHardwareDrivers.ps1
c:/windows/System32/nvidia-smi.exe
ffmpeg.exe -y -hwaccel cuda -hwaccel_output_format cuda -i test.mp4 -c:v h264_nvenc -b:v 5M test2.mp4
ffmpeg.exe -hwaccel d3d11va -i test.mp4 test4.mp4
