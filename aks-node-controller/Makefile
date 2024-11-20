.PHONY: proto-generate
proto-generate:
	docker build -t protoc-docker - < protoc.Dockerfile
	docker run --rm -v $(shell pwd):/$(shell pwd) --workdir=$(shell pwd) protoc-docker protoc --go_opt=module=github.com/Azure/agentbaker/aks-node-controller --go_out=./  --proto_path=proto  $(shell find proto/aksnodeconfig/v1 -name '*.proto')