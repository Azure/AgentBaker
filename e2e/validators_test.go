package e2e

import "testing"

// TestOpensslProviderActive exercises the parser used by ValidateFIPSProvider so we don't
// regress on real-world `openssl list -providers` output shapes — in particular, the
// AzureLinux V3 / ACL FIPS images expose the provider as "symcryptprovider" rather than
// "symcrypt" (ICM 51000001009688) and indentation has varied between distros.
func TestOpensslProviderActive(t *testing.T) {
	cases := []struct {
		name     string
		output   string
		prefixes []string
		want     bool
	}{
		{
			name: "symcryptprovider active on AzureLinux V3 matches symcrypt prefix",
			output: `Providers:
  default
    name: OpenSSL Default Provider
    version: 3.3.0
    status: active
  symcryptprovider
    name: SymCrypt Provider
    version: 103.4.2
    status: active
`,
			prefixes: []string{"fips", "symcrypt"},
			want:     true,
		},
		{
			name: "fips provider active",
			output: `Providers:
  default
    status: active
  fips
    name: OpenSSL FIPS Provider
    status: active
`,
			prefixes: []string{"fips", "symcrypt"},
			want:     true,
		},
		{
			name: "symcrypt provider inactive, default active does not satisfy",
			output: `Providers:
  default
    name: OpenSSL Default Provider
    status: active
  symcrypt
    name: SymCrypt Provider
    status: inactive
`,
			prefixes: []string{"fips", "symcrypt"},
			want:     false,
		},
		{
			name: "multiple active providers but no fips/symcrypt header at all",
			output: `Providers:
  default
    name: OpenSSL Default Provider
    status: active
  legacy
    name: OpenSSL Legacy Provider
    status: active
`,
			prefixes: []string{"fips", "symcrypt"},
			want:     false,
		},
		{
			name: "no fips or symcrypt provider listed",
			output: `Providers:
  default
    status: active
`,
			prefixes: []string{"fips", "symcrypt"},
			want:     false,
		},
		{
			name: "tolerates tab-indented provider header",
			output: "Providers:\n\tsymcryptprovider\n\t\tstatus: active\n",
			prefixes: []string{"fips", "symcrypt"},
			want:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := opensslProviderActive(tc.output, tc.prefixes...)
			if got != tc.want {
				t.Fatalf("opensslProviderActive() = %v, want %v\noutput:\n%s", got, tc.want, tc.output)
			}
		})
	}
}
