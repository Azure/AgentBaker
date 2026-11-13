package scenario

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWaagentEgress(t *testing.T) {
	out := `=== egress ===
     12 DST=168.63.129.16
      6 DST=20.60.140.4
=== waagent.log since restart ===
2026-11-04T02:46:07Z WARNING ExtHandler ExtHandler Fetch failed: [HttpError] [HTTP Failed] GET https://md-x.z4.blob.storage.azure.net/a -- IOError timed out
2026-11-04T02:46:07Z INFO ExtHandler ExtHandler Default channel changed to HostGAPlugin channel.
`
	e := parseWaagentEgress(out)
	require.Equal(t, map[string]int{"168.63.129.16": 12, "20.60.140.4": 6}, e.destinations)
	require.Equal(t, []string{"20.60.140.4 (6 SYN)"}, e.disallowed())
	require.Contains(t, e.logExcerpt, "Default channel changed")

	clean := parseWaagentEgress("=== egress ===\n      9 DST=168.63.129.16\n      1 DST=169.254.169.254\n=== waagent.log since restart ===\n")
	require.Empty(t, clean.disallowed())
	require.Empty(t, clean.logExcerpt)
}
