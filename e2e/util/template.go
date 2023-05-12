package util

import "fmt"

func GetDebugDaemonset() string {
	return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: &name debug
  namespace: default
  labels:
    app: *name
spec:
  replicas: 1
  selector:
    matchLabels:
      app: *name
  template:
    metadata:
      labels:
        app: *name
    spec:
      hostNetwork: true
      nodeSelector:
        kubernetes.azure.com/agentpool: nodepool1
      hostPID: true
      containers:
      - image: mcr.microsoft.com/oss/nginx/nginx:1.21.6
        name: ubuntu
        command: ["sleep", "infinity"]
        resources:
          requests: {}
          limits: {}
        securityContext:
          privileged: true
          capabilities:
            add: ["SYS_PTRACE", "SYS_RAWIO"]
`
}

func GetNginxPodTemplate(nodeName string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %[1]s-nginx
  namespace: default
spec:
  containers:
  - name: nginx
    image: mcr.microsoft.com/oss/nginx/nginx:1.21.6
    imagePullPolicy: IfNotPresent
  nodeSelector:
    kubernetes.io/hostname: %[1]s
`, nodeName)
}

func GetWasmSpinPodTemplate(nodeName string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %[1]s-wasm-spin
  namespace: default
spec:
  runtimeClassName: wasmtime-spin
  containers:
  - name: spin-hello
    image: ghcr.io/deislabs/containerd-wasm-shims/examples/spin-rust-hello:v0.5.1
    imagePullPolicy: IfNotPresent
    command: ["/"]
    resources: # limit the resources to 128Mi of memory and 100m of CPU
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 100m
        memory: 128Mi
  nodeSelector:
    kubernetes.io/hostname: %[1]s
`, nodeName)
}

func GetWasmSlightPodTemplate(nodeName string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %[1]s-wasm-slight
  namespace: default
spec:
  runtimeClassName: wasmtime-slight
  containers:
  - name: slight-hello
    image: ghcr.io/deislabs/containerd-wasm-shims/examples/slight-rust-hello:v0.5.1
    imagePullPolicy: IfNotPresent
    command: ["/"]
    resources: # limit the resources to 128Mi of memory and 100m of CPU
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 100m
        memory: 128Mi
  nodeSelector:
    kubernetes.io/hostname: %[1]s
`, nodeName)
}
