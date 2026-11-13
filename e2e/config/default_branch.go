package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (c *Configuration) resolveSIGVersionTag(ctx context.Context) error {
	if c.SIGVersionTagValue != "" || !strings.EqualFold(c.SIGVersionTagName, "branch") || c.VHDMetadataFile != "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 29*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--symref", "origin", "HEAD")
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.WaitDelay = time.Second
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return fmt.Errorf("resolve default SIG branch from origin: %w; set SIG_VERSION_TAG_VALUE explicitly to use a specific branch", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" &&
			strings.HasPrefix(fields[1], "refs/heads/") && fields[1] != "refs/heads/" {
			c.SIGVersionTagValue = fields[1]
			return nil
		}
	}
	return fmt.Errorf("origin did not advertise a default branch; set SIG_VERSION_TAG_VALUE explicitly to use a specific branch")
}
