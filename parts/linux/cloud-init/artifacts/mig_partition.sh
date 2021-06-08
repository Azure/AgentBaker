#!/bin/bash
GO111MODULE=off
go get -u github.com/NVIDIA/mig-parted/cmd/nvidia-mig-parted
GOBIN=$(pwd)
go install github.com/NVIDIA/mig-parted/cmd/nvidia-mig-parted@latest
git clone http://github.com/NVIDIA/mig-parted
cd ~/mig-parted
go build ./cmd/nvidia-mig-parted