module github.com/Azure/agentbaker/aks-node-controller

go 1.26.0

require (
	github.com/Azure/agentbaker v0.20240503.0
	github.com/Azure/agentbaker/aks-live-patching v0.20260831.0
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.1
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/google/go-cmp v0.7.0
	github.com/stretchr/testify v1.12.1
	github.com/urfave/cli/v3 v3.8.0
	golang.org/x/net v0.58.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.11
)

replace github.com/Azure/agentbaker => ../

require (
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)
