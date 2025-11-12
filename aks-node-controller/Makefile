# Run buf in docker, mounting the full repo into the container
# Emulate running "buf" in the current directory
BUF = docker run --volume "$(CURDIR)/../:$(CURDIR)/../" --workdir $(CURDIR) bufbuild/buf:1.47.2

.PHONY: proto-generate
proto-generate:
	@($(BUF) format -w)
	rm -rf pkg/gen/aksnodeconfig/v1
	docker build --platform $(shell uname -m) -t protoc-docker - < protoc.Dockerfile
	docker run --rm -v $(shell pwd):/$(shell pwd) --workdir=$(shell pwd) protoc-docker protoc --go_opt=module=github.com/Azure/agentbaker/aks-node-controller --go_out=./  --proto_path=proto  $(shell find proto/aksnodeconfig/v1 -name '*.proto')
	$(MAKE) proto-lint

.PHONY: proto-lint
proto-lint:
	@($(BUF) lint)
	@($(BUF) breaking --against '../.git#branch=master,subdir=aks-node-controller')
