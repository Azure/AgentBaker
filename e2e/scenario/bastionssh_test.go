package scenario

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

type bastionTestTransport func(*http.Request) (*http.Response, error)

func (f bastionTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type bastionTestCredential func(context.Context) (azcore.AccessToken, error)

func (f bastionTestCredential) GetToken(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return f(ctx)
}

func newTestBastion(t *testing.T, handleTunnel http.HandlerFunc) (*Bastion, *atomic.Int32) {
	t.Helper()
	var deleted atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			_, _ = io.WriteString(w, `{"authToken":"fixture-auth","websocketToken":"fixture-websocket","nodeId":"fixture-node"}`)
		case http.MethodDelete:
			assert.Equal(t, "/api/tokens/fixture-auth", r.URL.Path)
			assert.Equal(t, "fixture-node", r.Header.Get("X-Node-Id"))
			deleted.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			handleTunnel(w, r)
		}
	}))
	t.Cleanup(server.Close)
	bastion := NewBastion(bastionTestCredential(func(ctx context.Context) (azcore.AccessToken, error) {
		return azcore.AccessToken{Token: "fixture-token"}, ctx.Err()
	}), "subscription", "resource-group", strings.TrimPrefix(server.URL, "https://"))
	assert.Zero(t, bastion.httpClient.Timeout, "a client timeout would also expire the established WebSocket")
	bastion.httpClient = server.Client()
	bastion.httpTransport = server.Client().Transport.(*http.Transport)
	return bastion, &deleted
}

func TestBastionSSHSurvivesClearedHandshakeDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	bastion, deleted := newTestBastion(t, func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer ws.CloseNow()
		server, channels, requests, err := ssh.NewServerConn(websocket.NetConn(ctx, ws, websocket.MessageBinary), config)
		if !assert.NoError(t, err) {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			if err := serveSessionTestCommand(channel, func(_ string, channel ssh.Channel) uint32 {
				_, err := io.WriteString(channel, "hello\n")
				assert.NoError(t, err)
				return 0
			}); err != nil {
				t.Error(err)
			}
		}
	})
	dialCtx, cancelDial := context.WithCancel(ctx)
	defer cancelDial()
	tunnel, err := bastion.NewTunnelSession(dialCtx, "127.0.0.1", 22)
	require.NoError(t, err)
	defer tunnel.Close()
	deadline := time.Now().Add(200 * time.Millisecond)
	require.NoError(t, tunnel.SetDeadline(deadline))
	conn, channels, requests, err := ssh.NewClientConn(tunnel, "fixture", &ssh.ClientConfig{
		User: "test", HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()),
	})
	require.NoError(t, err)
	client := newSSHClient(ssh.NewClient(conn, channels, requests))
	defer client.Close()
	require.NoError(t, tunnel.SetDeadline(time.Time{}))
	cancelDial()
	time.Sleep(time.Until(deadline) + 20*time.Millisecond)
	result, err := runSSHCommand(ctx, client, "echo hello", false)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", result.stdout)
	assert.Equal(t, "0", result.exitCode)
	require.NoError(t, client.Close())
	require.NoError(t, tunnel.Close())
	assert.EqualValues(t, 1, deleted.Load())
}

func TestBastionSSHHandshakeCancellation(t *testing.T) {
	entered := make(chan struct{})
	bastion, deleted := newTestBastion(t, func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer ws.CloseNow()
		// Consume the client's SSH identification, but never send the server's.
		if _, _, err := ws.Read(r.Context()); err != nil {
			t.Error(err)
			return
		}
		close(entered)
		for {
			if _, _, err := ws.Read(r.Context()); err != nil {
				return
			}
		}
	})
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(key, "")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		client, err := DialSSHOverBastion(ctx, bastion, "127.0.0.1", pem.EncodeToMemory(block))
		if client != nil {
			_ = client.Close()
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("SSH handshake did not start")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
		assert.EqualValues(t, 1, deleted.Load())
	case <-time.After(time.Second):
		t.Fatal("SSH handshake ignored cancellation")
	}
}

func TestBastionDialFailureDeletesSession(t *testing.T) {
	bastion, deleted := newTestBastion(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	tunnel, err := bastion.NewTunnelSession(t.Context(), "127.0.0.1", 22)
	require.Error(t, err)
	assert.Nil(t, tunnel)
	assert.EqualValues(t, 1, deleted.Load())
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
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var deleted atomic.Int32
			conn, peer := net.Pipe()
			defer peer.Close()
			tunnel := &tunnelSession{
				Conn: conn, session: &sessionToken{},
				bastion: &Bastion{dnsName: "fixture.invalid", httpClient: &http.Client{
					Transport: bastionTestTransport(func(r *http.Request) (*http.Response, error) {
						assert.NoError(t, r.Context().Err())
						_, ok := r.Context().Deadline()
						assert.True(t, ok, "session deletion must have a timeout")
						deleted.Add(1)
						return &http.Response{StatusCode: status, Body: http.NoBody}, nil
					}),
				}},
			}
			var workers sync.WaitGroup
			for range 8 {
				workers.Go(func() {
					err := tunnel.Close()
					if status == http.StatusInternalServerError {
						assert.ErrorContains(t, err, "unexpected status code: 500")
					} else {
						assert.NoError(t, err)
					}
				})
			}
			workers.Wait()
			assert.EqualValues(t, 1, deleted.Load())
		})
	}
}
