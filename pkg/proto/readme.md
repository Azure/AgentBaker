Notes for protobuf

1. Run this command to compile .proto to .pb.go file.
```/AgentBaker/hack/tools/bin/buf generate -o .```

1. If you haven't installed buf yet, run the following command, assumed /usr/local/bin is where your Go binary locates.

```GO111MODULE=on GOBIN=/usr/local/bin go install github.com/bufbuild/buf/cmd/buf@v1.27.2```