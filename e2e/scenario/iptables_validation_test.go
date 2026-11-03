package scenario

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPTablesDHCPRulesCompatibleWithEBPFHostRouting(t *testing.T) {
	tablePatterns, globalPatterns := getIPTablesRulesCompatibleWithEBPFHostRouting()
	patterns := append(append([]string{}, globalPatterns...), tablePatterns["filter"]...)

	tests := []struct {
		name string
		rule string
		want bool
	}{
		{name: "DHCP INPUT", rule: "-A INPUT -p udp%s --dport 68 -j ACCEPT", want: true},
		{name: "different chain", rule: "-A OUTPUT -p udp%s --dport 68 -j ACCEPT"},
		{name: "different protocol", rule: "-A INPUT -p tcp --dport 68 -j ACCEPT"},
		{name: "different protocol and match module", rule: "-A INPUT -p tcp -m tcp --dport 68 -j ACCEPT"},
		{name: "different match module", rule: "-A INPUT -p udp -m tcp --dport 68 -j ACCEPT"},
		{name: "different port", rule: "-A INPUT -p udp%s --dport 67 -j ACCEPT"},
		{name: "port suffix", rule: "-A INPUT -p udp%s --dport 680 -j ACCEPT"},
		{name: "different target", rule: "-A INPUT -p udp%s --dport 68 -j DROP"},
		{name: "target suffix", rule: "-A INPUT -p udp%s --dport 68 -j ACCEPT_OTHER"},
		{name: "source condition", rule: "-A INPUT -s 10.0.0.0/8 -p udp%s --dport 68 -j ACCEPT"},
		{name: "interface condition", rule: "-A INPUT -i eth0 -p udp%s --dport 68 -j ACCEPT"},
		{name: "state condition", rule: "-A INPUT -p udp%s --dport 68 -m conntrack --ctstate NEW -j ACCEPT"},
		{name: "trailing condition", rule: "-A INPUT -p udp%s --dport 68 -j ACCEPT -s 10.0.0.0/8"},
		{name: "leading content", rule: "unexpected -A INPUT -p udp%s --dport 68 -j ACCEPT"},
	}

	for _, module := range []string{"", " -m udp"} {
		name := "implicit UDP match"
		if module != "" {
			name = "explicit UDP match"
		}
		t.Run(name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					rule := strings.ReplaceAll(tt.rule, "%s", module)
					matched := false
					for _, pattern := range patterns {
						regexMatch, err := regexp.MatchString(pattern, rule)
						require.NoError(t, err)
						if regexMatch || strings.Contains(rule, pattern) {
							matched = true
							break
						}
					}
					require.Equal(t, tt.want, matched, "rule: %s", rule)
				})
			}
		})
	}
}
