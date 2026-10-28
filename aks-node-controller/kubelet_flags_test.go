package main

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNBCFallbackClearsInheritedKubeletOmissions(test *testing.T) {
	for _, mode := range []string{"NBC", "compare"} {
		test.Run(mode, func(test *testing.T) {
			test.Setenv("KUBELET_FLAGS_TO_OMIT", base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"]`)))
			test.Setenv("PRESERVED_TEST_ENV", "preserved")
			scriptPath := filepath.Join(test.TempDir(), "nbc.sh")
			require.NoError(test, os.WriteFile(scriptPath, []byte(`printf 'omission=%s;other=%s' "${KUBELET_FLAGS_TO_OMIT:-}" "$PRESERVED_TEST_ENV"`), 0600))
			application := NewTestApp(test, TestAppConfig{RunFunc: cmdRunner})
			flags := ProvisionFlags{NBCCmd: scriptPath}
			if mode == "compare" {
				flags.ProvisionConfig = "parser/testdata/test_aksnodeconfig.json"
			}
			result, err := application.App.Provision(context.Background(), flags)
			require.NoError(test, err)
			require.Equal(test, "0", result.ExitCode)
			require.Contains(test, result.Output, "omission=;other=preserved")
		})
	}
}
