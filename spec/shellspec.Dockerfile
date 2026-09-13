# Dockerfile
# Don't forget to log in to the Azure Container Registry before building this image:
# az acr login --name aksdataplanedev
FROM aksdataplanedev.azurecr.io/shellspec/shellspec-debian:0.28.1
RUN sed -i -e 's/\(deb\|security\).debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    apt-get update &&  \
    apt-get install -y --no-install-recommends gawk jq curl dnsutils &&  \
    apt-get clean &&  \
    rm -rf /var/lib/apt/lists/*
# kcov (https://github.com/SimonKagstrom/kcov) provides code coverage for shellspec via its
# built-in `--kcov` flag. Debian buster doesn't ship a `kcov` apt package, so build it from
# source. Build tooling is purged afterwards; only the runtime libs it links against remain.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git ca-certificates build-essential cmake pkg-config python3 \
        binutils-dev libssl-dev libcurl4-openssl-dev libelf-dev zlib1g-dev libdw-dev && \
    git clone --depth 1 --branch v43 https://github.com/SimonKagstrom/kcov.git /tmp/kcov && \
    mkdir /tmp/kcov/build && cd /tmp/kcov/build && \
    cmake -DCMAKE_BUILD_TYPE=Release .. && \
    make -j"$(nproc)" && \
    make install && \
    cd / && rm -rf /tmp/kcov && \
    apt-get purge -y --auto-remove git ca-certificates build-essential cmake pkg-config python3 && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
COPY ./ /src
