# Dockerfile
# Don't forget to log in to the Azure Container Registry before building this image:
# az acr login --name aksdataplanedev
FROM aksdataplanedev.azurecr.io/shellspec/shellspec-debian:0.28.1
RUN apt-get -o DPkg::Lock::Timeout=60 update && apt-get install -o DPkg::Lock::Timeout=60 -y --no-install-recommends gawk jq curl && apt-get -o DPkg::Lock::Timeout=60 clean && rm -rf /var/lib/apt/lists/*
COPY ./ /src
