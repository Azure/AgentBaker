package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCSEScript(t *testing.T) {
	script, err := CSEScript(context.TODO(), Config{})
	require.NoError(t, err)
	require.NotEmpty(t, script)
}
