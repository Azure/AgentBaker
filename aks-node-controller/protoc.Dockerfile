# Use a multi-stage build to handle different architectures
FROM golang:1.23.3-alpine3.20

ARG TARGETARCH
RUN if [ "${TARGETARCH}" = "amd64" ]; then \
      wget -O protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v28.3/protoc-28.3-linux-x86_64.zip; \
    else \
      wget -O protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v28.3/protoc-28.3-linux-aarch_64.zip; \
    fi && \
    unzip protoc.zip -d /usr/local && \
    rm protoc.zip
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2

CMD ["protoc"]
