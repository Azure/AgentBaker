module github.com/Azure/agentbaker/aks-node-controller

go 1.23.7

require (
	github.com/Azure/agentbaker v0.20240503.0
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/fsnotify/fsnotify v1.8.0
	github.com/google/go-cmp v0.7.0
	github.com/stretchr/testify v1.11.1
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/aws/aws-sdk-go-v2 v1.38.2 // indirect
	github.com/clarketm/json v1.17.1 // indirect
	github.com/coreos/butane v0.25.1 // indirect
	github.com/coreos/go-json v0.0.0-20230131223807-18775e0fb4fb // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/coreos/ignition/v2 v2.23.0 // indirect
	github.com/coreos/vcontext v0.0.0-20230201181013-d72178a18687 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/vincent-petithory/dataurl v1.0.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/Azure/agentbaker => ../

replace github.com/coreos/ignition/v2 => github.com/flatcar/ignition/v2 v2.0.0-20250903113522-05b8a773288c
