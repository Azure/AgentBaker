FROM mcr.microsoft.com/windows/server:ltsc2022 AS runtime
WORKDIR /
COPY /ffmpeg-4.3.1-win64 ./
COPY /SetupHardwareDrivers.ps1 ./
COPY /gpuTest.bat ./
ENTRYPOINT ["gpuTest.bat"]
