module github.com/Azure/agentbaker

go 1.26.0

require (
	github.com/Azure/go-autorest/autorest/to v0.4.1
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/barkimedes/go-deepcopy v0.0.0-20220514131651-17c30cfc62df
	github.com/coreos/butane v0.29.0
	github.com/google/go-cmp v0.7.0
	github.com/gorilla/handlers v1.5.2
	github.com/gorilla/mux v1.8.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.41.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.10.2
	github.com/stretchr/testify v1.12.1
	github.com/vincent-petithory/dataurl v1.0.0
	gopkg.in/yaml.v3 v3.0.1
	mvdan.cc/sh/v3 v3.14.1
)

require (
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/aws/aws-sdk-go-v2 v1.47.8 // indirect
	github.com/clarketm/json v1.17.1 // indirect
	github.com/coreos/go-json v0.0.0-20231102161613-e49c8866685a // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/coreos/ignition/v2 v2.28.0 // indirect
	github.com/coreos/vcontext v0.0.0-20260306102053-7a68b5426c74 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/spf13/pflag v1.1.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
)

replace github.com/coreos/ignition/v2 => github.com/flatcar/ignition/v2 v2.0.0-20250903113522-05b8a773288c
