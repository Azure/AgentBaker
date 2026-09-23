# Dockerfile
# Don't forget to log in to the Azure Container Registry before building this image:
# az acr login --name aksdataplanedev

# kcov (https://github.com/SimonKagstrom/kcov) provides code coverage for shellspec via its
# built-in `--kcov` flag. Debian buster doesn't ship a `kcov` apt package, so build it from
# source in a separate stage: only the resulting binary is copied into the final image, so
# the `-dev` headers/static libs and build tooling never end up in the shipped image.
FROM aksdataplanedev.azurecr.io/shellspec/shellspec-debian:0.28.1 AS kcov-builder
# Pinned to the commit the v43 tag points to (verified via `git ls-remote --tags` /
# `gh api repos/SimonKagstrom/kcov/git/tags/<tag-sha>`). Still cloned by tag name (kcov's own
# build embeds its version via `git describe`, which needs the tag ref present), but the build
# fails outright if `v43` is ever moved/retagged to point somewhere else, so a moved tag can't
# silently change what gets built here.
ARG KCOV_COMMIT=a39874f938ce13f7a65f253120d1ec946b349ffe
RUN sed -i -e 's/\(deb\|security\).debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        git ca-certificates build-essential cmake pkg-config python3 \
        binutils-dev libssl-dev libcurl4-openssl-dev libelf-dev zlib1g-dev libdw-dev && \
    git clone --depth 1 --branch v43 https://github.com/SimonKagstrom/kcov.git /tmp/kcov && \
    cd /tmp/kcov && \
    [ "$(git rev-parse HEAD)" = "${KCOV_COMMIT}" ] && \
    mkdir /tmp/kcov/build && cd /tmp/kcov/build && \
    cmake -DCMAKE_BUILD_TYPE=Release .. && \
    make -j"$(nproc)" && \
    make install DESTDIR=/kcov-out

FROM aksdataplanedev.azurecr.io/shellspec/shellspec-debian:0.28.1
RUN sed -i -e 's/\(deb\|security\).debian.org/archive.debian.org/g' /etc/apt/sources.list && \
    apt-get update &&  \
    apt-get install -y --no-install-recommends \
        gawk jq curl dnsutils make openssl \
        libcurl4 libdw1 libelf1 libssl1.1 zlib1g &&  \
    apt-get clean &&  \
    rm -rf /var/lib/apt/lists/*
# Only the kcov binary itself is needed at runtime; the html-report assets are compiled in.
COPY --from=kcov-builder /kcov-out/usr/local/bin/kcov /usr/local/bin/kcov
COPY ./ /src
