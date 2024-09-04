module github.com/Azure/agentbaker/node-bootstrapper

go 1.23.0

replace github.com/Azure/agentbaker => ../

require (
	github.com/Azure/agentbaker v1.0.1238
	github.com/stretchr/testify v1.9.0
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.28.5 // indirect
)
