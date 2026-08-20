package e2e

import (
	"regexp"
	"testing"
)

// localdnsRawRules are raw-table rules exactly as `iptables -t raw -S` prints them on a node.
//
// The request-direction rules are produced by build_localdns_iptable_rules in
// parts/linux/cloud-init/artifacts/localdns.sh. The reply-direction rules are not produced by
// localdns.sh on main today, but they are baked into VHDs published while #9230 was on main,
// and those VHDs stay in support for 6 months. Both directions must therefore stay allowlisted.
var localdnsRawRules = []string{
	`-A PREROUTING -d 169.254.10.10/32 -p tcp -m comment --comment "localdns: skip conntrack" -m tcp --dport 53 -j NOTRACK`,
	`-A PREROUTING -d 169.254.10.10/32 -p udp -m comment --comment "localdns: skip conntrack" -m udp --dport 53 -j NOTRACK`,
	`-A PREROUTING -d 169.254.10.11/32 -p tcp -m comment --comment "localdns: skip conntrack" -m tcp --dport 53 -j NOTRACK`,
	`-A PREROUTING -d 169.254.10.11/32 -p udp -m comment --comment "localdns: skip conntrack" -m udp --dport 53 -j NOTRACK`,
	`-A OUTPUT -d 169.254.10.10/32 -p tcp -m comment --comment "localdns: skip conntrack" -m tcp --dport 53 -j NOTRACK`,
	`-A OUTPUT -d 169.254.10.10/32 -p udp -m comment --comment "localdns: skip conntrack" -m udp --dport 53 -j NOTRACK`,
	`-A OUTPUT -d 169.254.10.11/32 -p tcp -m comment --comment "localdns: skip conntrack" -m tcp --dport 53 -j NOTRACK`,
	`-A OUTPUT -d 169.254.10.11/32 -p udp -m comment --comment "localdns: skip conntrack" -m udp --dport 53 -j NOTRACK`,
	`-A OUTPUT -s 169.254.10.10/32 -p tcp -m comment --comment "localdns: skip conntrack" -m tcp --sport 53 -j NOTRACK`,
	`-A OUTPUT -s 169.254.10.10/32 -p udp -m comment --comment "localdns: skip conntrack" -m udp --sport 53 -j NOTRACK`,
	`-A OUTPUT -s 169.254.10.11/32 -p tcp -m comment --comment "localdns: skip conntrack" -m tcp --sport 53 -j NOTRACK`,
	`-A OUTPUT -s 169.254.10.11/32 -p udp -m comment --comment "localdns: skip conntrack" -m udp --sport 53 -j NOTRACK`,
}

// allowlistPatternsForTable mirrors how ValidateIPTablesCompatibleWithCiliumEBPF assembles the
// pattern set for a single table.
func allowlistPatternsForTable(table string) []string {
	tablePatterns, globalPatterns := getIPTablesRulesCompatibleWithEBPFHostRouting()
	return append(append([]string{}, globalPatterns...), tablePatterns[table]...)
}

// Test_iptablesAllowlist_patternsCompile guards against a malformed entry in the allowlist. Such
// an entry never matches, so it would otherwise surface as an unrelated "unsupported iptables
// rule" failure in every scenario.
func Test_iptablesAllowlist_patternsCompile(t *testing.T) {
	tablePatterns, globalPatterns := getIPTablesRulesCompatibleWithEBPFHostRouting()

	assertCompiles := func(scope string, patterns []string) {
		t.Helper()
		for _, pattern := range patterns {
			if _, err := regexp.Compile(pattern); err != nil {
				t.Errorf("%s pattern %q does not compile: %v", scope, pattern, err)
			}
		}
	}

	assertCompiles("global", globalPatterns)
	for table, patterns := range tablePatterns {
		assertCompiles(table, patterns)
	}
}

// Test_iptablesAllowlist_localdnsRules pins the LocalDNS NOTRACK rules to the raw-table allowlist.
// Dropping either direction turns every Linux scenario red, because ValidateCommonLinux runs
// ValidateIPTablesCompatibleWithCiliumEBPF against whatever localdns.sh the booted VHD baked in.
func Test_iptablesAllowlist_localdnsRules(t *testing.T) {
	patterns := allowlistPatternsForTable("raw")

	for _, rule := range localdnsRawRules {
		matched, err := iptablesRuleMatchesAnyPattern(rule, patterns)
		if err != nil {
			t.Fatalf("matching rule %q: %v", rule, err)
		}
		if !matched {
			t.Errorf("LocalDNS rule is not allowlisted for eBPF host routing: %s", rule)
		}
	}
}

// Test_iptablesAllowlist_rejectsUnknownRule proves the allowlist still fails closed, so the test
// above cannot be satisfied by an overly broad pattern.
func Test_iptablesAllowlist_rejectsUnknownRule(t *testing.T) {
	patterns := allowlistPatternsForTable("raw")
	rule := `-A OUTPUT -d 10.0.0.1/32 -p tcp -m tcp --dport 8080 -j DROP`

	matched, err := iptablesRuleMatchesAnyPattern(rule, patterns)
	if err != nil {
		t.Fatalf("matching rule %q: %v", rule, err)
	}
	if matched {
		t.Errorf("expected unknown rule to be rejected, but it matched: %s", rule)
	}
}

func Test_iptablesRuleMatchesAnyPattern(t *testing.T) {
	testCases := []struct {
		name      string
		rule      string
		patterns  []string
		wantMatch bool
		wantErr   bool
	}{
		{
			name:      "regex pattern matches",
			rule:      `-A OUTPUT -s 169.254.10.10/32 --sport 53 -j NOTRACK`,
			patterns:  []string{`^-A OUTPUT -s 169\.254\.10\.(10|11)/32 --sport 53 -j NOTRACK$`},
			wantMatch: true,
		},
		{
			// "[bar]" is a character class as a regex, so this entry can only match literally.
			name:      "literal substring matches when the regex does not",
			rule:      `-A OUTPUT -m set --match-set foo[bar] src -j DROP`,
			patterns:  []string{`--match-set foo[bar] src`},
			wantMatch: true,
		},
		{
			name:      "no pattern matches",
			rule:      `-A OUTPUT -j DROP`,
			patterns:  []string{`^-A INPUT`},
			wantMatch: false,
		},
		{
			name:      "empty patterns are skipped",
			rule:      `-A OUTPUT -j DROP`,
			patterns:  []string{"", "   "},
			wantMatch: false,
		},
		{
			name:     "invalid pattern is reported when nothing matches",
			rule:     `-A OUTPUT -j DROP`,
			patterns: []string{`^-A (OUTPUT`},
			wantErr:  true,
		},
		{
			// An entry pasted verbatim out of `iptables -S` need not be a valid regex. It must
			// still get its literal-substring chance, and must not be reported as an error once
			// the rule is accounted for.
			name:      "invalid pattern still matches literally",
			rule:      `-A OUTPUT -m comment --comment "kube-proxy (v1.31" -j ACCEPT`,
			patterns:  []string{`--comment "kube-proxy (v1.31"`},
			wantMatch: true,
		},
		{
			name:      "invalid pattern is not reported when a later pattern matches",
			rule:      `-A OUTPUT -j DROP`,
			patterns:  []string{`^-A (OUTPUT`, `^-A OUTPUT -j DROP$`},
			wantMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matched, err := iptablesRuleMatchesAnyPattern(tc.rule, tc.patterns)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for patterns %v, got none", tc.patterns)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if matched != tc.wantMatch {
				t.Errorf("got match=%v, want %v", matched, tc.wantMatch)
			}
		})
	}
}
