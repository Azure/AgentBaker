package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/Azure/agentbaker/e2e/toolkit"
)

var AllowedSSHPrefixes = []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSA, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512}

type Bastion struct {
	credential                                 azcore.TokenCredential
	subscriptionID, resourceGroupName, dnsName string
	httpClient                                 *http.Client
	httpTransport                              *http.Transport
}

func NewBastion(credential *azidentity.AzureCLICredential, subscriptionID, resourceGroupName, dnsName string) *Bastion {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     30 * time.Second,
	}

	return &Bastion{
		credential:        credential,
		subscriptionID:    subscriptionID,
		resourceGroupName: resourceGroupName,
		dnsName:           dnsName,
		httpTransport:     transport,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

type tunnelSession struct {
	bastion *Bastion
	ws      *websocket.Conn
	session *sessionToken
	ctx     context.Context

	readDeadline  time.Time
	writeDeadline time.Time
	readBuf       []byte

	targetHost string
	targetPort uint16
	closeOnce  sync.Once
	closeErr   error
}

func (b *Bastion) NewTunnelSession(ctx context.Context, targetHost string, port uint16) (*tunnelSession, error) {
	session, err := b.newSessionToken(ctx, targetHost, port)
	if err != nil {
		return nil, err
	}

	wsUrl := fmt.Sprintf("wss://%v/webtunnelv2/%v?X-Node-Id=%v", b.dnsName, session.WebsocketToken, session.NodeID)

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	ws, _, err := websocket.Dial(dialCtx, wsUrl, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	cancel()
	if err != nil {
		b.deleteSessionTokenAsync(ctx, session)
		return nil, err
	}

	ws.SetReadLimit(32 * 1024 * 1024)

	return &tunnelSession{
		bastion:    b,
		ws:         ws,
		session:    session,
		ctx:        ctx,
		targetHost: targetHost,
		targetPort: port,
	}, nil
}

type sessionToken struct {
	AuthToken            string   `json:"authToken"`
	Username             string   `json:"username"`
	DataSource           string   `json:"dataSource"`
	NodeID               string   `json:"nodeId"`
	AvailableDataSources []string `json:"availableDataSources"`
	WebsocketToken       string   `json:"websocketToken"`
}

func (t *tunnelSession) Close() error {
	t.closeOnce.Do(func() {
		_ = t.ws.CloseNow()
		if t.ctx.Err() != nil {
			t.bastion.deleteSessionTokenAsync(t.ctx, t.session)
		} else {
			t.closeErr = t.bastion.deleteSessionToken(t.ctx, t.session)
		}
	})
	return t.closeErr
}

func (b *Bastion) deleteSessionTokenAsync(ctx context.Context, session *sessionToken) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	go func() {
		defer cancel()
		if err := b.deleteSessionToken(cleanupCtx, session); err != nil {
			toolkit.Logf(cleanupCtx, "Failed to delete bastion session: %v", err)
		}
	}()
}

func (b *Bastion) deleteSessionToken(ctx context.Context, session *sessionToken) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://%v/api/tokens/%v", b.dnsName, session.AuthToken), nil)
	if err != nil {
		return err
	}

	req.Header.Add("X-Node-Id", session.NodeID)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		var requestErr *url.Error
		if errors.As(err, &requestErr) {
			return requestErr.Err
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	if b.httpTransport != nil {
		b.httpTransport.CloseIdleConnections()
	}

	return nil
}

func (b *Bastion) newSessionToken(ctx context.Context, targetHost string, port uint16) (*sessionToken, error) {

	token, err := b.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{fmt.Sprintf("%s/.default", cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint)},
	})

	if err != nil {
		return nil, err
	}

	apiUrl := fmt.Sprintf("https://%v/api/tokens", b.dnsName)

	// target_resource_id = f"/subscriptions/{get_subscription_id(cmd.cli_ctx)}/resourceGroups/{resource_group_name}/providers/Microsoft.Network/bh-hostConnect/{target_ip_address}"
	data := url.Values{}
	data.Set("resourceId", fmt.Sprintf("/subscriptions/%v/resourceGroups/%v/providers/Microsoft.Network/bh-hostConnect/%v", b.subscriptionID, b.resourceGroupName, targetHost))
	data.Set("protocol", "tcptunnel")
	data.Set("workloadHostPort", fmt.Sprintf("%v", port))
	data.Set("aztoken", token.Token)
	data.Set("hostname", targetHost)

	req, err := http.NewRequestWithContext(ctx, "POST", apiUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := b.httpClient.Do(req) // TODO client settings
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("error creating tunnel: %v", resp.Status)
	}

	var response sessionToken

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (t *tunnelSession) Read(p []byte) (int, error) {
	if len(t.readBuf) == 0 {
		ctx := t.ctx
		if !t.readDeadline.IsZero() {
			var cancel context.CancelFunc
			ctx, cancel = context.WithDeadline(t.ctx, t.readDeadline)
			defer cancel()
		}
		typ, data, err := t.ws.Read(ctx)
		if err != nil {
			return 0, err
		}
		if typ != websocket.MessageBinary {
			return 0, fmt.Errorf("unexpected websocket message type: %v", typ)
		}
		t.readBuf = data
	}

	n := copy(p, t.readBuf)
	t.readBuf = t.readBuf[n:]
	return n, nil
}

func (t *tunnelSession) Write(p []byte) (int, error) {
	ctx := t.ctx
	if !t.writeDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(t.ctx, t.writeDeadline)
		defer cancel()
	}
	if err := t.ws.Write(ctx, websocket.MessageBinary, p); err != nil {
		return 0, err
	}

	return len(p), nil
}

func (t *tunnelSession) LocalAddr() net.Addr {
	return bastionAddr{
		network: "bastion",
		address: "local",
	}
}

func (t *tunnelSession) RemoteAddr() net.Addr {
	return bastionAddr{
		network: "bastion",
		address: fmt.Sprintf("%s:%d", t.targetHost, t.targetPort),
	}
}

func (t *tunnelSession) SetDeadline(deadline time.Time) error {
	t.readDeadline = deadline
	t.writeDeadline = deadline
	return nil
}

func (t *tunnelSession) SetReadDeadline(deadline time.Time) error {
	t.readDeadline = deadline
	return nil
}

func (t *tunnelSession) SetWriteDeadline(deadline time.Time) error {
	t.writeDeadline = deadline
	return nil
}

type bastionAddr struct {
	network string
	address string
}

func (a bastionAddr) Network() string { return a.network }
func (a bastionAddr) String() string  { return a.address }

func sshClientConfig(user string, privateKey []byte) (*ssh.ClientConfig, error) {
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyAlgorithms: AllowedSSHPrefixes,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if !slices.Contains(AllowedSSHPrefixes, key.Type()) {
				return fmt.Errorf("unexpected host key type: %s", key.Type())
			}
			return nil
		},
	}, nil
}

func DialSSHOverBastion(
	ctx context.Context,
	bastion *Bastion,
	vmPrivateIP string,
	sshPrivateKey []byte,
) (*ssh.Client, error) {
	sshConfig, err := sshClientConfig("azureuser", sshPrivateKey)
	if err != nil {
		return nil, err
	}

	return dialSSHOverBastion(ctx, vmPrivateIP, sshConfig, func(ctx context.Context) (net.Conn, error) {
		return bastion.NewTunnelSession(ctx, vmPrivateIP, 22)
	})
}

func dialSSHOverBastion(
	ctx context.Context,
	vmPrivateIP string,
	sshConfig *ssh.ClientConfig,
	openTunnel func(context.Context) (net.Conn, error),
) (*ssh.Client, error) {
	const (
		sshReadinessTimeout = 5 * time.Minute
		sshDialBackoff      = 10 * time.Second
	)
	ctx, cancel := context.WithTimeout(ctx, sshReadinessTimeout)
	defer cancel()
	start := time.Now()
	deadline, _ := ctx.Deadline()

	var lastErr error
	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			err := ctx.Err()
			if err == nil {
				err = context.DeadlineExceeded
			}
			return nil, fmt.Errorf("SSH readiness to %s ended after %s: %w (last attempt: %v)", vmPrivateIP, time.Since(start), err, lastErr)
		}
		toolkit.Logf(ctx, "Attempt %d establishing SSH over bastion to %s (elapsed %s)", attempt, vmPrivateIP, time.Since(start))

		client, err := dialSSHAttempt(ctx, vmPrivateIP, sshConfig, openTunnel)
		if err == nil {
			toolkit.Logf(ctx, "SSH over bastion to %s ready after %s (%d attempts)", vmPrivateIP, time.Since(start), attempt)
			return client, nil
		}
		lastErr = err
		toolkit.Logf(ctx, "Attempt %d SSH over bastion failed after %s: %v", attempt, time.Since(start), err)
		if ctx.Err() != nil {
			continue
		}
		if !isTransientSSHError(err) {
			return nil, err
		}
		timer := time.NewTimer(sshDialBackoff)
		select {
		case <-timer.C:
		case <-ctx.Done():
		}
		timer.Stop()
	}
}

func dialSSHAttempt(
	ctx context.Context,
	address string,
	config *ssh.ClientConfig,
	openTunnel func(context.Context) (net.Conn, error),
) (*ssh.Client, error) {
	tunnelCtx, cancelTunnel := context.WithCancel(context.WithoutCancel(ctx))
	stopReadiness := context.AfterFunc(ctx, cancelTunnel)
	defer stopReadiness()

	conn, err := openTunnel(tunnelCtx)
	if err != nil {
		cancelTunnel()
		return nil, fmt.Errorf("open bastion tunnel: %w", err)
	}
	tunnel := &sshReadinessConn{Conn: conn, cancel: cancelTunnel}

	handshakeCtx, cancelHandshake := context.WithTimeout(ctx, 30*time.Second)
	handshakeDeadline, _ := handshakeCtx.Deadline()
	stopHandshake := context.AfterFunc(handshakeCtx, cancelTunnel)
	sshConn, chans, reqs, err := ssh.NewClientConn(tunnel, address, config)
	stopHandshake()
	stopReadiness()
	handshakeErr := handshakeCtx.Err()
	if handshakeErr == nil && !time.Now().Before(handshakeDeadline) {
		handshakeErr = context.DeadlineExceeded
	}
	cancelHandshake()
	if handshakeErr != nil {
		err = handshakeErr
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		if cleanupErr := tunnel.Close(); cleanupErr != nil {
			toolkit.Logf(ctx, "Failed to close bastion tunnel: %v", cleanupErr)
		}
		return nil, fmt.Errorf("SSH handshake: %w", err)
	}
	tunnel.ready.Store(true)
	return ssh.NewClient(sshConn, chans, reqs), nil
}

type sshReadinessConn struct {
	net.Conn
	cancel   context.CancelFunc
	close    sync.Once
	closeErr error
	ready    atomic.Bool
}

func (c *sshReadinessConn) Close() error {
	c.close.Do(func() {
		defer c.cancel()
		if !c.ready.Load() {
			c.cancel()
		}
		c.closeErr = c.Conn.Close()
	})
	return c.closeErr
}

func isTransientSSHError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	switch websocket.CloseStatus(err) {
	case websocket.StatusGoingAway, websocket.StatusAbnormalClosure, websocket.StatusInternalError,
		websocket.StatusServiceRestart, websocket.StatusTryAgainLater:
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EHOSTUNREACH) ||
		errors.Is(err, syscall.ENETUNREACH)
}
