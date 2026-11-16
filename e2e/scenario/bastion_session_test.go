package scenario

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bastionTestTransport func(*http.Request) (*http.Response, error)

func (f bastionTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type bastionTestCredential func(context.Context) (azcore.AccessToken, error)

func (f bastionTestCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return f(ctx)
}

func TestBastionSessionCreationHonorsCancellation(t *testing.T) {
	for _, stage := range []string{"credential", "token-request"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			entered := make(chan struct{})
			block := func(ctx context.Context) error {
				close(entered)
				<-ctx.Done()
				return ctx.Err()
			}
			bastion := NewBastion(bastionTestCredential(func(ctx context.Context) (azcore.AccessToken, error) {
				if stage == "credential" {
					return azcore.AccessToken{}, block(ctx)
				}
				return azcore.AccessToken{Token: "fixture-token"}, nil
			}), "subscription", "resource-group", "fixture.invalid")
			bastion.httpClient.Transport = bastionTestTransport(func(r *http.Request) (*http.Response, error) {
				return nil, block(r.Context())
			})
			done := make(chan error, 1)
			go func() {
				_, err := bastion.NewTunnelSession(ctx, "127.0.0.1", 22)
				done <- err
			}()
			<-entered
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(time.Second):
				t.Fatal("Bastion session creation ignored cancellation at " + stage)
			}
		})
	}
}

func TestBastionSessionCloseDeletesOnce(t *testing.T) {
	var deleted atomic.Int32
	conn, peer := net.Pipe()
	defer peer.Close()
	tunnel := &tunnelSession{
		Conn: conn, session: &sessionToken{},
		bastion: &Bastion{dnsName: "fixture.invalid", httpClient: &http.Client{
			Transport: bastionTestTransport(func(r *http.Request) (*http.Response, error) {
				deleted.Add(1)
				return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
			}),
		}},
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() { assert.NoError(t, tunnel.Close()) })
	}
	workers.Wait()
	assert.Eventually(t, func() bool { return deleted.Load() == 1 }, time.Second, time.Millisecond)
}

func TestBastionSessionDeletionStatus(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			bastion := &Bastion{dnsName: "fixture.invalid", httpClient: &http.Client{
				Transport: bastionTestTransport(func(r *http.Request) (*http.Response, error) {
					assert.NoError(t, r.Context().Err())
					_, ok := r.Context().Deadline()
					assert.True(t, ok, "session deletion must have a timeout")
					return &http.Response{StatusCode: status, Body: http.NoBody}, nil
				}),
			}}
			err := bastion.deleteSession(&sessionToken{})
			if status == http.StatusInternalServerError {
				assert.ErrorContains(t, err, "unexpected status code: 500")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
