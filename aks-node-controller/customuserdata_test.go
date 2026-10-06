package main

import (
	"context"
	"testing"
)

func TestCustomUserDataCommandsRejectInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"aks-node-controller", "apply-custom-user-data", "unexpected"},
		{"aks-node-controller", "reconcile-custom-user-data"},
		{"aks-node-controller", "reconcile-custom-user-data", "--node-name=--invalid"},
	} {
		if code := (&App{}).Run(context.Background(), args); code == 0 {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
}
