package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeCloudLocation(t *testing.T) {
	location, err := normalizeCloudLocation(" US Sec East \t\n")
	require.NoError(t, err)
	require.Equal(t, "usseceast", location)
	for _, value := range []string{`westus";echo bad`, "westus%bad", "westus\\bad"} {
		_, err = normalizeCloudLocation(value)
		require.Error(t, err)
	}
}

func TestCloudCertificateRefreshCommand(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		err       error
		wantCall  bool
		wantError bool
	}{
		{"valid", []string{"--location", "usnateast"}, nil, true, false},
		{"refresh failure", []string{"--location", "usnateast"}, errors.New("refresh failed"), true, true},
		{"missing location", nil, nil, false, true},
		{"empty location", []string{"--location", " "}, nil, false, true},
		{"unexpected argument", []string{"--location", "usnateast", "extra"}, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			called := false
			tt.App.refreshCloudCertificates = func(_ context.Context, location string) error {
				called = true
				require.Equal(t, "usnateast", location)
				return tc.err
			}
			code := tt.App.Run(t.Context(), append([]string{"aks-node-controller", "refresh-cloud-certificates"}, tc.args...))
			require.Equal(t, tc.wantCall, called)
			require.Equal(t, tc.wantError, code != 0)
		})
	}
}
