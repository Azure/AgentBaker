FROM mcr.microsoft.com/azurelinux/base/core:3.0

# Define build-time arguments for the protobuf versions
ARG PROTOC_VERSION=28.3
ARG PROTOC_GEN_GO_VERSION=1.35.2

# Determine architecture and set appropriate URLs
RUN set -e; \
    ARCH=`uname -m`; \
    tdnf install -y wget unzip tar ca-certificates; \
    if [ "$ARCH" = "x86_64" ]; then \
    PROTOC_URL="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip"; \
    PROTOC_GEN_GO_URL="https://github.com/protocolbuffers/protobuf-go/releases/download/v${PROTOC_GEN_GO_VERSION}/protoc-gen-go.v${PROTOC_GEN_GO_VERSION}.linux.amd64.tar.gz"; \
    elif [ "$ARCH" = "aarch64" ]; then \
    PROTOC_URL="https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-aarch_64.zip"; \
    PROTOC_GEN_GO_URL="https://github.com/protocolbuffers/protobuf-go/releases/download/v${PROTOC_GEN_GO_VERSION}/protoc-gen-go.v${PROTOC_GEN_GO_VERSION}.linux.arm64.tar.gz"; \
    else \
    echo "Unsupported architecture: $ARCH" && exit 1; \
    fi; \
    \
    # Download and install protobuf compiler
    wget -O protoc.zip $PROTOC_URL; \
    unzip protoc.zip -d /usr/local; \
    rm protoc.zip; \
    \
    # Download and install protobuf Go plugin
    wget -O protoc-gen-go.tar.gz $PROTOC_GEN_GO_URL; \
    tar -xzf protoc-gen-go.tar.gz -C /usr/local/bin; \
    rm protoc-gen-go.tar.gz

# Default command
CMD ["protoc"]
