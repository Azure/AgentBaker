package scenario

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSysctlOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:     "empty value",
			output:   "net.ipv4.ip_local_reserved_ports = \n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": ""},
		},
		{
			name:     "nonempty value does not match empty",
			output:   "net.ipv4.ip_local_reserved_ports = 65330\n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": ""},
			wantErr:  true,
		},
		{
			name:     "missing empty value",
			output:   "net.ipv4.ip_local_port_range = 32768\t65535\n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": ""},
			wantErr:  true,
		},
		{
			name:   "multiple values with kernel whitespace",
			output: "net.ipv4.ip_local_port_range = 32768\t65535\nnet.netfilter.nf_conntrack_max = 2097152\n",
			expected: map[string]string{
				"net.ipv4.ip_local_port_range":   "32768 65535",
				"net.netfilter.nf_conntrack_max": "2097152",
			},
		},
		{
			name:     "numeric prefix does not match",
			output:   "net.netfilter.nf_conntrack_max = 20971520\n",
			expected: map[string]string{"net.netfilter.nf_conntrack_max": "2097152"},
			wantErr:  true,
		},
		{
			name:     "port list prefix does not match",
			output:   "net.ipv4.ip_local_reserved_ports = 65330,65331\n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": "65330"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSysctlOutput(tt.output, tt.expected)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
