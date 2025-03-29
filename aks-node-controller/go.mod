module github.com/Azure/agentbaker/aks-node-controller

go 1.23.7

require (
	github.com/Azure/agentbaker v0.20240503.0
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.17.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/fsnotify/fsnotify v1.8.0
	github.com/google/go-cmp v0.7.0
	github.com/stretchr/testify v1.10.0
	google.golang.org/protobuf v1.35.2
)

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.1 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/sys v0.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/Azure/agentbaker => ../
