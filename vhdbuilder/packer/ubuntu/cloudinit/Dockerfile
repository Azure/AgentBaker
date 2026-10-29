ARG UBUNTU_RELEASE
FROM ubuntu:${UBUNTU_RELEASE} AS build
ARG UBUNTU_RELEASE
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl dpkg-dev devscripts build-essential
WORKDIR /build
RUN set -eu; \
    case "$UBUNTU_RELEASE" in \
      24.04) version='26.2-0ubuntu1~24.04.2'; checksum='03196fece7142691b4c9a46149b53edae070ccc1c5d844d2e499f04cee717e08'; packaging_checksum='2b80fb834e60f47ef9f313dc77c45f7b77d47903101b8f2bf9b24e9d78c24c88' ;; \
      26.04) version='26.2-0ubuntu1~26.04.1'; checksum='26e37a0f3da0108d0e1021c17aa70389ccb51261f09cfd3ae492053cf0dbae53'; packaging_checksum='1781c18146661fda65a3ca1d4df299fd0043e3ea7e76f01f6d2761802d26e5bd' ;; \
      *) echo "Unsupported Ubuntu release: $UBUNTU_RELEASE" >&2; exit 1 ;; \
    esac; \
    base="https://launchpad.net/ubuntu/+archive/primary/+sourcefiles/cloud-init/$version"; \
    curl -fSL --retry 3 "$base/cloud-init_$version.dsc" -o cloud-init.dsc; \
    echo "$checksum  cloud-init.dsc" | sha256sum -c -; \
    curl -fSL --retry 3 "$base/cloud-init_26.2.orig.tar.gz" -o cloud-init_26.2.orig.tar.gz; \
    curl -fSL --retry 3 "$base/cloud-init_$version.debian.tar.xz" -o "cloud-init_$version.debian.tar.xz"; \
    echo '43d03aad9095c0c0a78a3b67d37c657a3da75a3b2ea8bbb03b7b4eb5240c8ee3  cloud-init_26.2.orig.tar.gz' | sha256sum -c -; \
    echo "$packaging_checksum  cloud-init_$version.debian.tar.xz" | sha256sum -c -; \
    dpkg-source --no-check -x cloud-init.dsc source
WORKDIR /build/source
RUN apt-get build-dep -y --no-install-recommends .
COPY runtime-datasource-options.patch /build/
RUN patch --batch --fuzz=0 -p1 < /build/runtime-datasource-options.patch
RUN python3 -m pytest -q tests/unittests/sources/test_azure.py \
    tests/unittests/sources/test_azure_runtime_options.py
RUN DEBFULLNAME='AKS Node Lifecycle' DEBEMAIL='aks-node-lifecycle@users.noreply.github.com' \
    dch --newversion "$(dpkg-parsechangelog --show-field Version)+aks2" \
    'Allow boolean skip-ready overrides in Azure cloud-config and cloud-config-archive CustomData.'
RUN DEB_BUILD_OPTIONS=nocheck dpkg-buildpackage -us -uc -b
RUN mkdir /output && cp /build/*.deb /output/ && \
    cd /output && sha256sum ./*.deb > SHA256SUMS

FROM scratch
COPY --from=build /output/ /
