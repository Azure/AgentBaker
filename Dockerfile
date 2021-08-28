# FROM golang:1.17 as builder

# WORKDIR /workspace
# # Copy the Go Modules manifests
# COPY go.mod go.mod
# COPY go.sum go.sum

# # cache deps before building and copying source so that we don't need to re-download as much
# # and so that source changes don't invalidate our downloaded layer
# RUN go mod download

# COPY cmd cmd
# COPY pkg pkg

# RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o baker ./cmd/agentbaker

FROM gcr.io/distroless/base:nonroot
# COPY --from=builder /workspace/baker /usr/local/bin/baker
COPY baker /usr/local/bin/baker

ENTRYPOINT [ "/usr/local/bin/baker" ]
