# test/node-problem-detector/docker/Dockerfile
FROM mcr.microsoft.com/devcontainers/base:ubuntu AS base

ENV DEBIAN_FRONTEND=noninteractive

# Install all required tools
RUN apt-get update && apt-get install -y \
    sysstat \
    bc \
    coreutils \
    procps \
    jq \
    iotop \
    ethtool \
    iproute2 \
    python3 \
    dnsutils \
    iptables \
    systemctl \
    && rm -rf /var/lib/apt/lists/*

FROM base AS without-ig

# Create required directories including events directory
RUN mkdir -p /config/node-problem-detector/plugin \
    /var/log/azure/Microsoft.AKS.Compute.AKS.Linux.AKSNode/events \
    /var/log/events \
    /mock-commands \
    /usr/local/bin \
    /etc/node-problem-detector.d/custom-plugin-monitor \
    /etc/node-problem-detector.d/system-log-monitor \
    /etc/node-problem-detector.d/system-stats-monitor \
    /etc/node-problem-detector.d/plugin

# Copy ALL plugin scripts to the location monitor JSONs expect
COPY parts/linux/cloud-init/artifacts/node-problem-detector/plugin/ /etc/node-problem-detector.d/plugin/

# Copy NPD configuration files
COPY parts/linux/cloud-init/artifacts/node-problem-detector/custom-plugin-monitor/ /etc/node-problem-detector.d/custom-plugin-monitor/
COPY parts/linux/cloud-init/artifacts/node-problem-detector/system-log-monitor/ /etc/node-problem-detector.d/system-log-monitor/
COPY parts/linux/cloud-init/artifacts/node-problem-detector/system-stats-monitor/ /etc/node-problem-detector.d/system-stats-monitor/

# Copy the NPD startup script
COPY parts/linux/cloud-init/artifacts/node-problem-detector-startup.sh /usr/local/bin/

RUN chmod +x /etc/node-problem-detector.d/plugin/*.sh /etc/node-problem-detector.d/plugin/*.sh /usr/local/bin/node-problem-detector-startup.sh

# Create mock commands for missing tools
RUN echo '#!/bin/bash\necho "mock systemd-cgtop output"' > /usr/bin/systemd-cgtop && chmod +x /usr/bin/systemd-cgtop
RUN echo '#!/bin/bash\necho "mock crictl output"' > /usr/bin/crictl && chmod +x /usr/bin/crictl

# Create mock node-problem-detector for startup script tests
RUN echo '#!/bin/bash\necho "node-problem-detector started successfully with args: $@"' > /usr/local/bin/node-problem-detector && chmod +x /usr/local/bin/node-problem-detector

# Override PATH to use our mock commands first
ENV PATH="/mock-commands:$PATH"
ENV EVENTS_LOG_DIR="/var/log/events"

WORKDIR /config/node-problem-detector/plugin

# Optional stage that installs Inspektor Gadget and its dependencies
FROM without-ig AS with-ig
COPY test/node-problem-detector/fixtures/mock-containerd/bin/mock-containerd /usr/local/bin/mock-containerd
RUN chmod +x /usr/local/bin/mock-containerd
COPY dependencies/artifacts/ig*amd64.deb /tmp/ig.deb
RUN dpkg -i /tmp/ig.deb && rm -f /tmp/ig.deb
