module github.com/Azure/agentbaker/aks-node-controller

go 1.25.11

require (
	github.com/Azure/agentbaker/aks-live-patching v0.0.0-00010101000000-000000000000
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.1
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/google/go-cmp v0.7.0
	github.com/stretchr/testify v1.11.1
	github.com/urfave/cli/v3 v3.8.0
	golang.org/x/sync v0.20.0
	golang.org/x/sys v0.45.0
	google.golang.org/grpc v1.82.1
	google.golang.org/protobuf v1.36.11
	gopkg.in/yaml.v3 v3.0.1
	software.sslmate.com/src/go-pkcs12 v0.7.3
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace github.com/Azure/agentbaker => ../

replace github.com/Azure/agentbaker/aks-live-patching => ../aks-live-patching

replace github.com/coreos/ignition/v2 => github.com/flatcar/ignition/v2 v2.0.0-20250903113522-05b8a773288c
