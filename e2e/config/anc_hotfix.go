package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ANCHotfixVersions struct {
	Base   string
	Target string
}

// Ordinary runs exercise a known published package; release validation must supply its own.
var defaultANCHotfixVersions = ANCHotfixVersions{Base: "202608.21.0", Target: "202608.21.1"}

var ancHotfixVersionPattern = regexp.MustCompile(`^[0-9]{6}\.[0-9]{2}\.[1-9][0-9]*$`)

// ResolveANCHotfixVersions runs after CLI parsing, before scenario setup or Azure initialization.
func (c *Configuration) ResolveANCHotfixVersions() error {
	target := c.ANCHotfixTargetVersion
	if target == "" {
		if c.RequireANCHotfixValidation {
			return fmt.Errorf("ANC_HOTFIX_TARGET_VERSION is required when REQUIRE_ANC_HOTFIX_VALIDATION=true")
		}
		c.ANCHotfixVersions = defaultANCHotfixVersions
		return nil
	}
	invalid := func() error {
		return fmt.Errorf("invalid ANC_HOTFIX_TARGET_VERSION %q: expected YYYYMM.DD.PATCH with a valid date and positive patch", target)
	}
	if !ancHotfixVersionPattern.MatchString(target) {
		return invalid()
	}
	parts := strings.Split(target, ".")
	base := parts[0] + "." + parts[1]
	if date, err := time.Parse("200601.02", base); err != nil || date.Year() < 1 {
		return invalid()
	}
	if _, err := strconv.ParseUint(parts[2], 10, 64); err != nil {
		return invalid()
	}
	c.ANCHotfixVersions = ANCHotfixVersions{Base: base + ".0", Target: target}
	return nil
}
