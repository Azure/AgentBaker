.PHONY: proto-generate lint-proto-files
proto-generate:
	docker build --platform $(shell uname -m) -t protoc-docker - < protoc.Dockerfile
	docker run --rm -v $(shell pwd):/$(shell pwd) --workdir=$(shell pwd) protoc-docker protoc --go_opt=module=github.com/Azure/agentbaker/aks-node-controller --go_out=./  --proto_path=proto  $(shell find proto/aksnodeconfig/v1 -name '*.proto')
	$(MAKE) proto-lint

# Run buf in docker, mounting the full repo into the container
# Emulate running "buf" in the current directory
BUF = docker run --volume "$(CURDIR)/../:$(CURDIR)/../" --workdir $(CURDIR) bufbuild/buf:1.47.2

proto-lint:
	@($(BUF) lint)
	@($(BUF) breaking --against '../.git#branch=dev,subdir=aks-node-controller') # TODO: change to master
