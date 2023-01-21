#!/usr/bin/env bash

# This script originated at https://github.com/kubernetes/kubernetes/blob/master/cluster/gce/gci/health-monitor.sh
# and has been modified for aks-engine.

set -o nounset
set -o pipefail

container_runtime_monitoring() {
  local -r max_attempts=5
  local attempt=1
  local -r container_runtime_name=$1

  if [[ ${container_runtime_name} == "containerd" ]]; then
    local healthcheck_command="ctr --namespace k8s.io container list"
  else 
    local healthcheck_command="docker ps"
  fi

  until timeout 60 ${healthcheck_command} > /dev/null; do
    if (( attempt == max_attempts )); then
      echo "Max attempt ${max_attempts} reached! Proceeding to monitor container runtime healthiness."
      break
    fi
    echo "$attempt initial attempt \"${healthcheck_command}\"! Trying again in $attempt seconds..."
    sleep "$(( 2 ** attempt++ ))"
  done
  while true; do
    if ! timeout 60 ${healthcheck_command} > /dev/null; then
      echo "Container runtime ${container_runtime_name} failed!"
      if [[ "$container_runtime_name" == "containerd" ]]; then
        pkill -SIGUSR1 containerd
      else 
        pkill -SIGUSR1 dockerd
      fi
      systemctl kill --kill-who=main "${container_runtime_name}"
      sleep 120
    else
      sleep "${SLEEP_SECONDS}"
    fi
  done
}

kubelet_monitoring() {
  echo "Wait for 2 minutes for kubelet to be functional"
  sleep 120
  local -r max_seconds=10
  local output=""
  while true; do
    if ! output=$(curl -m "${max_seconds}" -f -s -S http://127.0.0.1:10255/healthz 2>&1); then
      echo $output
      echo "Kubelet is unhealthy!"
      systemctl kill kubelet
      sleep 60
    else
      sleep "${SLEEP_SECONDS}"
    fi
  done
}

if [[ "$#" -lt 1 ]]; then
  echo "Usage: health-monitor.sh <container-runtime/kubelet>"
  exit 1
fi

component=$1
if [[ "${component}" == "container-runtime" ]]; then
  if [[ -z $2 ]]; then
    echo "Usage: health-monitor.sh container-runtime <docker/containerd>"
    exit 1
  fi
  container_runtime=$2
fi

KUBE_HOME="/usr/local/bin"
KUBE_ENV="/etc/default/kube-env"
if [[  -e "${KUBE_ENV}" ]]; then
  source "${KUBE_ENV}"
fi

SLEEP_SECONDS=10

echo "Start kubernetes health monitoring for ${component}"

if [[ "${component}" == "container-runtime" ]]; then
  container_runtime_monitoring ${container_runtime}
elif [[ "${component}" == "kubelet" ]]; then
  kubelet_monitoring
else
  echo "Health monitoring for component ${component} is not supported!"
fi
