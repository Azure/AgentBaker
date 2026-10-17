package scenario

import (
	"strings"
	"testing"
)

func TestFindIPEAuditDenial(t *testing.T) {
	probe := "/opt/ab-ipe-audit.1234567890/probe"
	record := `audit: type=1420 ipe_op=EXECUTE ipe_hook=BPRM_CHECK enforcing=0 pid=42 comm="bash" path="` + probe + `" dev="sda4" ino=123 rule="DEFAULT action=DENY"`
	for _, tt := range []struct {
		name    string
		records string
		want    bool
	}{
		{"audit denial", record, true},
		{"allow decision", strings.ReplaceAll(record, "action=DENY", "action=ALLOW"), false},
		{"enforcing denial", strings.ReplaceAll(record, "enforcing=0", "enforcing=1"), false},
		{"wrong operation", strings.ReplaceAll(record, "ipe_op=EXECUTE", "ipe_op=POLICY"), false},
		{"different probe", strings.ReplaceAll(record, "1234567890", "9876543210"), false},
		{"path prefix", strings.ReplaceAll(record, `/probe"`, `/probe-other"`), false},
		{"fields on separate records", strings.ReplaceAll(record, " enforcing=0", "\nenforcing=0"), false},
		{"no event", "", false},
		{"event among unrelated output", "unrelated\n" + record + "\nunrelated", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := findIPEAuditDenial(tt.records, probe)
			if (got != "") != tt.want {
				t.Fatalf("findIPEAuditDenial() = %q, want match %v", got, tt.want)
			}
		})
	}
}
