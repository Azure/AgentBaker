package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"

	"github.com/Azure/agentbaker/e2e/toolkit"
)

var AllowedSSHPrefixes = []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSA, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512}

type Bastion struct {
	credential                                 *azidentity.AzureCLICredential
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
}

func (b *Bastion) NewTunnelSession(ctx context.Context, targetHost string, port uint16) (*tunnelSession, error) {
	session, err := b.newSessionToken(targetHost, port)
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
	_ = t.ws.Close(websocket.StatusNormalClosure, "")

	req, err := http.NewRequest("DELETE", fmt.Sprintf("https://%v/api/tokens/%v", t.bastion.dnsName, t.session.AuthToken), nil)
	if err != nil {
		return err
	}

	req.Header.Add("X-Node-Id", t.session.NodeID)

	resp, err := t.bastion.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil
	}

	if resp.StatusCode != 204 {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	if t.bastion.httpTransport != nil {
		t.bastion.httpTransport.CloseIdleConnections()
	}

	return nil
}

func (b *Bastion) newSessionToken(targetHost string, port uint16) (*sessionToken, error) {

	token, err := b.credential.GetToken(context.Background(), policy.TokenRequestOptions{
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

	req, err := http.NewRequest("POST", apiUrl, strings.NewReader(data.Encode()))
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
		Timeout: 5 * time.Second,
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

	const (
		sshDialAttempts = 5
		sshDialTimeout  = 30 * time.Second
		sshDialBackoff  = 10 * time.Second
	)

	var lastErr error
	for attempt := 1; attempt <= sshDialAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-time.After(sshDialBackoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		toolkit.Logf(ctx, "Attempt %d/%d establishing SSH over bastion to %s", attempt, sshDialAttempts, vmPrivateIP)

		// Intentionally use a background context to prevent cancelling the SSH connection before
		// we fetch logs during cleanup.
		tunnel, err := bastion.NewTunnelSession(context.Background(), vmPrivateIP, 22)
		if err != nil {
			lastErr = err
			toolkit.Logf(ctx, "Attempt %d/%d failed to create bastion tunnel: %v", attempt, sshDialAttempts, err)
			continue
		}

		_ = tunnel.SetDeadline(time.Now().Add(sshDialTimeout))
		sshConn, chans, reqs, err := ssh.NewClientConn(
			tunnel,
			vmPrivateIP,
			sshConfig,
		)
		if err != nil {
			lastErr = err
			toolkit.Logf(ctx, "Attempt %d/%d SSH handshake failed: %v", attempt, sshDialAttempts, err)
			_ = tunnel.Close()
			continue
		}
		_ = tunnel.SetDeadline(time.Time{})
		return ssh.NewClient(sshConn, chans, reqs), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to establish SSH connection over bastion")
	}
	return nil, lastErr
}
