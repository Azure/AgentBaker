#!/bin/bash

# node could very well be running both dockerd and containerd. we just care about the runtime that kubelet is using.
function getKubeletRuntime() {
  CONTAINER_RUNTIME_ENDPOINT="${CONTAINER_RUNTIME_ENDPOINT:-unix:///run/containerd/containerd.sock}"
  if [[ "${CONTAINER_RUNTIME_ENDPOINT}" == *containerd.sock ]]; then
    echo "containerd"
  else 
    echo "docker"
  fi
}
