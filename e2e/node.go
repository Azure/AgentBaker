package e2e

import (
	"context"
	"fmt"

	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	wasmHandlerSpin = "spin"
)

func ensureWasmRuntimeClasses(ctx context.Context, kube *Kubeclient) error {
	// Only create spin class for now
	spinClassName := fmt.Sprintf("wasmtime-%s", wasmHandlerSpin)
	if err := createRuntimeClass(ctx, kube, spinClassName, wasmHandlerSpin); err != nil {
		return err
	}
	return nil
}

func createRuntimeClass(ctx context.Context, kube *Kubeclient, name, handler string) error {
	runtimeClass := &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, kube.Dynamic, runtimeClass, func() error {
		if runtimeClass.ObjectMeta.CreationTimestamp.IsZero() {
			runtimeClass.Handler = handler
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create spin runtime classs: %w", err)
	}

	return nil
}
