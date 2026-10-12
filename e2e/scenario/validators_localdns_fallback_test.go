package scenario

import "testing"

// Real busybox 2.0 nslookup output shapes. The parser exists because the exit code
// cannot be trusted (busybox returns 0 on NXDOMAIN in several builds) and because
// the server's own address is printed before the answer, so "contains Address" would
// pass on a failed lookup.
func TestNativeLookupAnswered(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  string
		want bool
	}{
		{
			name: "cluster name resolved through the cluster listener",
			out: `Server:		169.254.10.11
Address:	169.254.10.11:53

Name:      kubernetes.default.svc.cluster.local
Address 1: 10.0.0.1 kubernetes.default.svc.cluster.local
`,
			want: true,
		},
		{
			name: "external name resolved through the node listener",
			out: `Server:		169.254.10.10
Address:	169.254.10.10:53

Non-authoritative answer:
Name:      mcr.microsoft.com
Address 1: 204.79.197.219
`,
			want: true,
		},
		{
			// The failure this parser exists for: the server block is present and
			// contains "Address:", so a naive substring check reports success.
			name: "NXDOMAIN still prints the server address",
			out: `Server:		169.254.10.11
Address:	169.254.10.11:53

*** Can't find nosuchname.invalid: No answer
`,
			want: false,
		},
		{
			name: "no answer at all when the listener is dark",
			out: `;; connection timed out; no servers could be reached
`,
			want: false,
		},
		{
			// A truncated or interrupted stream must not read as success. This is the
			// kube.go:42 shape -- client-go returning without an exit status.
			name: "empty output",
			out:  "",
			want: false,
		},
		{
			name: "server block only, stream cut before the answer",
			out: `Server:		169.254.10.11
Address:	169.254.10.11:53
`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeLookupAnswered(tc.out); got != tc.want {
				t.Errorf("nativeLookupAnswered() = %v, want %v\noutput:\n%s", got, tc.want, tc.out)
			}
		})
	}
}
