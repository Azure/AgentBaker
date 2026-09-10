package e2e

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/Azure/agentbaker/e2e/toolkit"
)

func TestSSHReadinessRetriesBeyondFiveAttempts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config, serverConfig := readinessSSHConfigs(t)
		start := time.Now()
		attempts := 0
		client, err := dialSSHOverBastion(context.Background(), "vm", config, func(ctx context.Context) (net.Conn, error) {
			attempts++
			if attempts <= 5 {
				return nil, fmt.Errorf("open tunnel: %w", syscall.ECONNREFUSED)
			}
			return readinessSSHServer(t, ctx, serverConfig), nil
		})
		require.NoError(t, err)
		defer client.Close()
		assert.Equal(t, 6, attempts)
		assert.Equal(t, 50*time.Second, time.Since(start))
	})
}

func TestSSHReadinessOverallDeadline(t *testing.T) {
	for _, phase := range []string{"tunnel", "handshake", "backoff"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				config, _ := readinessSSHConfigs(t)
				start := time.Now()
				attempts := 0
				var tunnels []*readinessTestConn
				client, err := dialSSHOverBastion(context.Background(), "vm", config, func(ctx context.Context) (net.Conn, error) {
					attempts++
					switch phase {
					case "tunnel":
						<-ctx.Done()
						return nil, ctx.Err()
					case "handshake":
						conn, peer := net.Pipe()
						t.Cleanup(func() { _ = peer.Close() })
						tunnel := readinessContextConn(ctx, conn)
						tunnels = append(tunnels, tunnel)
						return tunnel, nil
					default:
						return nil, syscall.ECONNREFUSED
					}
				})
				require.Nil(t, client)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Equal(t, 5*time.Minute, time.Since(start))
				if phase == "tunnel" {
					assert.Equal(t, 1, attempts)
				}
				if phase == "handshake" {
					assert.Equal(t, 8, attempts)
				}
				if phase == "backoff" {
					assert.Equal(t, 30, attempts)
				}
				for _, tunnel := range tunnels {
					assert.Equal(t, int32(1), tunnel.closes.Load())
				}
			})
		})
	}
}

func TestSSHReadinessCallerCancellation(t *testing.T) {
	for _, phase := range []string{"before-open", "tunnel", "handshake", "backoff"} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/deadline=%v", phase, deadline), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					config, _ := readinessSSHConfigs(t)
					ctx, cancel := context.WithCancel(context.Background())
					wantErr := context.Canceled
					wait := 3 * time.Second
					if deadline {
						cancel()
						ctx, cancel = context.WithTimeout(context.Background(), wait)
						wantErr = context.DeadlineExceeded
					} else {
						time.AfterFunc(wait, cancel)
					}
					defer cancel()
					if phase == "before-open" {
						cancel()
						wait = 0
						wantErr = context.Canceled
					}
					start := time.Now()
					attempts := 0
					client, err := dialSSHOverBastion(ctx, "vm", config, func(ctx context.Context) (net.Conn, error) {
						attempts++
						switch phase {
						case "tunnel":
							<-ctx.Done()
							return nil, ctx.Err()
						case "handshake":
							conn, peer := net.Pipe()
							t.Cleanup(func() { _ = peer.Close() })
							return readinessContextConn(ctx, conn), nil
						default:
							return nil, syscall.ECONNREFUSED
						}
					})
					require.Nil(t, client)
					require.ErrorIs(t, err, wantErr)
					assert.Equal(t, wait, time.Since(start))
					if phase == "before-open" {
						assert.Zero(t, attempts)
					} else {
						assert.Equal(t, 1, attempts)
					}
				})
			})
		}
	}
}

func TestSSHReadinessTunnelSetupConsumesBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config, _ := readinessSSHConfigs(t)
		start := time.Now()
		attempts := 0
		_, err := dialSSHOverBastion(context.Background(), "vm", config, func(ctx context.Context) (net.Conn, error) {
			attempts++
			time.Sleep(280 * time.Second)
			conn, peer := net.Pipe()
			t.Cleanup(func() { _ = peer.Close() })
			return readinessContextConn(ctx, conn), nil
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 5*time.Minute, time.Since(start))
		assert.Equal(t, 1, attempts)
	})
}

func TestSSHReadinessCancellationAtHandshakeCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config, serverConfig := readinessSSHConfigs(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		config.HostKeyCallback = func(string, net.Addr, ssh.PublicKey) error {
			cancel()
			return nil
		}
		var tunnel *readinessTestConn
		client, err := dialSSHOverBastion(ctx, "vm", config, func(ctx context.Context) (net.Conn, error) {
			tunnel = readinessSSHServer(t, ctx, serverConfig)
			return tunnel, nil
		})
		require.Nil(t, client)
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), tunnel.closes.Load())
	})
}

func TestSSHReadinessPermanentHandshakeErrors(t *testing.T) {
	for _, failure := range []string{"authentication", "host-key", "host-key-algorithm"} {
		t.Run(failure, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				config, serverConfig := readinessSSHConfigs(t)
				wantMessage := ""
				switch failure {
				case "authentication":
					serverConfig.NoClientAuth = false
					serverConfig.PublicKeyCallback = func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
						return nil, errors.New("unauthorized key")
					}
					wantMessage = "unable to authenticate"
				case "host-key":
					config.HostKeyCallback = func(string, net.Addr, ssh.PublicKey) error {
						return errors.New("host key rejected")
					}
					wantMessage = "host key rejected"
				case "host-key-algorithm":
					config.HostKeyAlgorithms = []string{ssh.KeyAlgoRSA}
					wantMessage = "no common algorithm"
				}
				start := time.Now()
				attempts := 0
				var tunnel *readinessTestConn
				client, err := dialSSHOverBastion(context.Background(), "vm", config, func(ctx context.Context) (net.Conn, error) {
					attempts++
					tunnel = readinessSSHServer(t, ctx, serverConfig)
					return tunnel, nil
				})
				require.Nil(t, client)
				require.ErrorContains(t, err, wantMessage)
				assert.Equal(t, 1, attempts)
				assert.Zero(t, time.Since(start))
				assert.Equal(t, int32(1), tunnel.closes.Load())
			})
		})
	}
}

func TestSSHReadinessErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		transient bool
	}{
		{"connection refused", syscall.ECONNREFUSED, true},
		{"connection reset", syscall.ECONNRESET, true},
		{"EOF", io.EOF, true},
		{"unexpected EOF", io.ErrUnexpectedEOF, true},
		{"wrapped timeout", fmt.Errorf("ssh: handshake failed: failed to get reader: %w", context.DeadlineExceeded), true},
		{"network timeout", &net.DNSError{IsTimeout: true}, true},
		{"Bastion restart", websocket.CloseError{Code: websocket.StatusServiceRestart}, true},
		{"Bastion going away", websocket.CloseError{Code: websocket.StatusGoingAway}, true},
		{"Bastion policy rejection", websocket.CloseError{Code: websocket.StatusPolicyViolation}, false},
		{"Bastion protocol error", websocket.CloseError{Code: websocket.StatusProtocolError}, false},
		{"DNS not found", &net.DNSError{IsNotFound: true}, false},
		{"cancellation", context.Canceled, false},
		{"untyped timeout text", errors.New("invalid config: context deadline exceeded"), false},
		{"permission", errors.New("error creating tunnel: 403 Forbidden"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				config, serverConfig := readinessSSHConfigs(t)
				start := time.Now()
				attempts := 0
				client, err := dialSSHOverBastion(context.Background(), "vm", config, func(ctx context.Context) (net.Conn, error) {
					attempts++
					if attempts == 1 {
						return nil, tc.err
					}
					return readinessSSHServer(t, ctx, serverConfig), nil
				})
				if tc.transient {
					require.NoError(t, err)
					defer client.Close()
					assert.Equal(t, 2, attempts)
					assert.Equal(t, 10*time.Second, time.Since(start))
				} else {
					require.Nil(t, client)
					require.ErrorIs(t, err, tc.err)
					assert.Equal(t, 1, attempts)
					assert.Zero(t, time.Since(start))
				}
			})
		})
	}
}

func TestSSHReadinessSuccessfulClientSurvivesCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		config, serverConfig := readinessSSHConfigs(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var tunnel *readinessTestConn
		var tunnelCtx context.Context
		client, err := dialSSHOverBastion(ctx, "vm", config, func(ctx context.Context) (net.Conn, error) {
			tunnelCtx = ctx
			tunnel = readinessSSHServer(t, ctx, serverConfig)
			return tunnel, nil
		})
		require.NoError(t, err)
		defer client.Close()
		cancel()
		time.Sleep(6 * time.Minute)
		require.NoError(t, tunnelCtx.Err())
		ok, _, err := client.SendRequest("keepalive@test", true, nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, client.Close())
		var closing sync.WaitGroup
		for range 10 {
			closing.Go(func() { _ = client.Close() })
		}
		closing.Wait()
		assert.Equal(t, int32(1), tunnel.closes.Load())
		require.ErrorIs(t, tunnelCtx.Err(), context.Canceled)
	})
}

func TestBastionTokenCleanupOutlivesReadiness(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		ctx, cancel := context.WithCancel(toolkit.ContextWithLogger(context.Background(), t))
		cancel()
		var calls atomic.Int32
		done := make(chan struct{})
		bastion := &Bastion{
			dnsName: "bastion.invalid",
			httpClient: &http.Client{Transport: readinessRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				assert.Equal(t, http.MethodDelete, req.Method)
				assert.NoError(t, req.Context().Err())
				assert.Equal(t, toolkit.LoggerFromContext(ctx), toolkit.LoggerFromContext(req.Context()))
				<-req.Context().Done()
				close(done)
				return nil, req.Context().Err()
			})},
		}
		bastion.deleteSessionTokenAsync(ctx, &sessionToken{AuthToken: "test"})
		synctest.Wait()
		assert.Equal(t, int32(1), calls.Load())
		assert.Zero(t, time.Since(start))
		time.Sleep(30 * time.Second)
		<-done
		synctest.Wait()
		assert.Equal(t, 30*time.Second, time.Since(start))
	})
}

func TestBastionTunnelCloseIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		<-conn.CloseRead(r.Context()).Done()
	}))
	defer server.Close()
	ws, _, err := websocket.Dial(t.Context(), server.URL, nil)
	require.NoError(t, err)
	defer ws.CloseNow()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	deleted := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	tunnel := &tunnelSession{
		ws: ws, ctx: ctx, session: &sessionToken{AuthToken: "test"},
		bastion: &Bastion{
			dnsName: "bastion.invalid",
			httpClient: &http.Client{Transport: readinessRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				assert.NoError(t, req.Context().Err())
				close(deleted)
				select {
				case <-release:
				case <-req.Context().Done():
					return nil, req.Context().Err()
				}
				return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
			})},
		},
	}
	var closing sync.WaitGroup
	for range 10 {
		closing.Go(func() { assert.NoError(t, tunnel.Close()) })
	}
	closing.Wait()
	select {
	case <-deleted:
	case <-time.After(5 * time.Second):
		t.Fatal("token deletion was not attempted")
	}
	assert.Equal(t, int32(1), calls.Load())
}

func TestBastionSessionTokenHonorsCancellation(t *testing.T) {
	for _, phase := range []string{"credential", "HTTP"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				bastion := &Bastion{
					dnsName: "bastion.invalid",
					credential: readinessCredential(func(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
						if phase == "credential" {
							<-ctx.Done()
							return azcore.AccessToken{}, ctx.Err()
						}
						return azcore.AccessToken{Token: "test"}, nil
					}),
					httpClient: &http.Client{Transport: readinessRoundTripper(func(req *http.Request) (*http.Response, error) {
						<-req.Context().Done()
						return nil, req.Context().Err()
					})},
				}
				start := time.Now()
				_, err := bastion.NewTunnelSession(ctx, "vm", 22)
				require.ErrorIs(t, err, context.DeadlineExceeded)
				assert.Equal(t, time.Second, time.Since(start))
			})
		})
	}
}

type readinessCredential func(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error)

func (f readinessCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return f(ctx, options)
}

type readinessRoundTripper func(*http.Request) (*http.Response, error)

func (f readinessRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type readinessTestConn struct {
	net.Conn
	stop   func() bool
	once   sync.Once
	closes atomic.Int32
}

func readinessContextConn(ctx context.Context, conn net.Conn) *readinessTestConn {
	c := &readinessTestConn{Conn: conn}
	c.stop = context.AfterFunc(ctx, func() { _ = conn.Close() })
	return c
}

func (c *readinessTestConn) Close() error {
	c.once.Do(func() {
		c.stop()
		c.closes.Add(1)
		_ = c.Conn.Close()
	})
	return nil
}

func readinessSSHConfigs(t *testing.T) (*ssh.ClientConfig, *ssh.ServerConfig) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	server := &ssh.ServerConfig{NoClientAuth: true}
	server.AddHostKey(signer)
	return &ssh.ClientConfig{
		User:            "test",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, server
}

type readinessBufferedConn struct {
	net.Conn
	writes chan []byte
	done   chan struct{}
	once   sync.Once
}

func TestReadinessBufferedConnDrainsBeforeClose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		conn, peer := net.Pipe()
		defer peer.Close()
		buffered := newReadinessBufferedConn(ctx, conn)
		_, err := buffered.Write([]byte("first"))
		require.NoError(t, err)
		_, err = buffered.Write([]byte("second"))
		require.NoError(t, err)
		go buffered.Close()
		synctest.Wait()
		data, err := io.ReadAll(peer)
		require.NoError(t, err)
		assert.Equal(t, "firstsecond", string(data))
	})
}

func (c *readinessBufferedConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	select {
	case c.writes <- bytes.Clone(p):
		return len(p), nil
	case <-c.done:
		return 0, net.ErrClosed
	}
}

func (c *readinessBufferedConn) Close() error {
	c.once.Do(func() {
		select {
		case c.writes <- nil:
		case <-c.done:
		}
		<-c.done
		_ = c.Conn.Close()
	})
	return nil
}

func newReadinessBufferedConn(ctx context.Context, conn net.Conn) *readinessBufferedConn {
	buffered := &readinessBufferedConn{Conn: conn, writes: make(chan []byte, 16), done: make(chan struct{})}
	go func() {
		defer close(buffered.done)
		for {
			select {
			case data := <-buffered.writes:
				if data == nil {
					return
				}
				if _, err := conn.Write(data); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return buffered
}

func readinessSSHServer(t *testing.T, ctx context.Context, config *ssh.ServerConfig) *readinessTestConn {
	t.Helper()
	client, server := net.Pipe()
	buffered := newReadinessBufferedConn(ctx, server)
	go func() {
		defer buffered.Close()
		conn, channels, requests, err := ssh.NewServerConn(buffered, config)
		if err != nil {
			return
		}
		defer conn.Close()
		go func() {
			for channel := range channels {
				_ = channel.Reject(ssh.Prohibited, "not used by readiness tests")
			}
		}()
		for request := range requests {
			_ = request.Reply(true, nil)
		}
	}()
	return readinessContextConn(ctx, client)
}
