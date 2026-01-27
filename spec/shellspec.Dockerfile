# Dockerfile
# Don't forget to log in to the Azure Container Registry before building this image:
# az acr login --name aksdataplanedev
FROM aksdataplanedev.azurecr.io/shellspec/shellspec-debian:0.28.1
RUN sed -i -e 's/\(deb\|security\).debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    apt-get update &&  \
    apt-get install -y --no-install-recommends gawk jq curl &&  \
    apt-get clean &&  \
    rm -rf /var/lib/apt/lists/*
COPY ./ /src
