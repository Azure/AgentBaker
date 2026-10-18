package scenario

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestScriptUploadDeadlineClosesBlockedTransfer(t *testing.T) {
	release := make(chan struct{})
	client := newSessionTestSSHClient(t, func(ch ssh.NewChannel) error {
		channel, requests, err := ch.Accept()
		if err != nil {
			return err
		}
		defer channel.Close()
		request := <-requests
		if err := request.Reply(true, nil); err != nil {
			return err
		}
		<-release
		return nil
	})
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err := uploadSSHScript(ctx, client.Client, strings.Repeat("x\n", 2*1024*1024), true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
