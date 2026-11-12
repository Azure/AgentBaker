package scenario

import (
	"context"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func TestValidateNodeCanRunAPodRejectsUnknownWindowsImage(t *testing.T) {
	s := &Scenario{}
	s.VHD = &config.Image{Name: "windows-unknown", OS: config.OSWindows}

	err := ValidateNodeCanRunAPod(context.Background(), s)
	require.ErrorContains(t, err, "servercore")
	require.ErrorContains(t, err, "windows-unknown")
}
