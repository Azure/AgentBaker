package e2e_test

import (
	"context"
	"fmt"

	nodev1 "k8s.io/api/node/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	wasmSpinRuntime   = "spin"
	wasmSlightRuntime = "slight"
)

func ensureWasmRuntimeClasses(ctx context.Context, kube *kubeclient) error {
	spinClass := getWasmRuntimeClassTemplate(wasmSpinRuntime)
	if err := applyRuntimeClassManifest(ctx, kube, spinClass); err != nil {
		return fmt.Errorf("unable to apply wasm spin RuntimeClass: %w", err)
	}

	return nil
}

func applyRuntimeClassManifest(ctx context.Context, kube *kubeclient, manifest string) error {
	var runtimeClassObj nodev1.RuntimeClass
	if err := yaml.Unmarshal([]byte(manifest), &runtimeClassObj); err != nil {
		return fmt.Errorf("failed to unmarshal RuntimeClass manifest: %w", err)
	}

	desired := runtimeClassObj.DeepCopy()
	_, err := controllerutil.CreateOrUpdate(ctx, kube.dynamic, &runtimeClassObj, func() error {
		runtimeClassObj = *desired
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to apply RuntimeClass manifest: %w", err)
	}

	return nil
}
