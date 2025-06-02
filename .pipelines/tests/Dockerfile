FROM mcr.microsoft.com/windows/servercore:ltsc2019

#FROM containerrollingregistry.azurecr.io/serverdatacentercore:latest
LABEL maintainer="yuazhang@microsoft.com"

SHELL ["cmd"]

#RUN mkdir c:\SiloEtwMonitor
# Copy the monagent files into the container.
Copy .\\startMonAgent.ps1 c:/
#COPY .\\SiloEtwMonitor\\ c:/SiloEtwMonitor/

RUN mkdir c:\Geneva
#Please note the format here
#This will do recursive copy and keep the tree structure
COPY .\\Monitoring\\ c:/Geneva/

# Start a busywork command to keep the container running.
#ENTRYPOINT ["c:\SiloEtwMonitor\SiloEtwMonitor.exe c:\SiloEtwMonitor\SiloEtwMonitor.json"]