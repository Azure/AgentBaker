package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/coder/websocket"
	"golang.org/x/crypto/ssh"
)

var AllowedSSHPrefixes = []string{ssh.KeyAlgoED25519, ssh.KeyAlgoRSA, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512}

type Bastion struct {
	credential                                 azcore.TokenCredential
	subscriptionID, resourceGroupName, dnsName string
	httpClient                                 *http.Client
	httpTransport                              *http.Transport
}

func NewBastion(credential azcore.TokenCredential, subscriptionID, resourceGroupName, dnsName string) *Bastion {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
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
		// Use request contexts for timeouts, not a client timeout that can expire a WebSocket.
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

type tunnelSession struct {
	net.Conn
	bastion   *Bastion
	session   *sessionToken
	closeOnce sync.Once
	closeErr  error
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
		HTTPClient:      b.httpClient,
	})
	cancel()
	if err != nil {
		return nil, errors.Join(err, b.deleteSession(session))
	}

	return &tunnelSession{
		// Keep established connections available for cleanup after the dial context expires.
		Conn:    websocket.NetConn(context.Background(), ws, websocket.MessageBinary),
		bastion: b,
		session: session,
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
		t.closeErr = errors.Join(t.Conn.Close(), t.bastion.deleteSession(t.session))
	})
	return t.closeErr
}

func (b *Bastion) deleteSession(session *sessionToken) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b.httpTransport != nil {
		defer b.httpTransport.CloseIdleConnections()
	}
	req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("https://%v/api/tokens/%v", b.dnsName, session.AuthToken), nil)
	if err != nil {
		return err
	}

	req.Header.Add("X-Node-Id", session.NodeID)

	resp, err := b.httpClient.Do(req)
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

	return nil
}

func (b *Bastion) newSessionToken(ctx context.Context, targetHost string, port uint16) (*sessionToken, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
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
	resp, err := b.httpClient.Do(req)
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
) (*SSHClient, error) {
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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 1 {
			select {
			case <-time.After(sshDialBackoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		logging.Logf(ctx, "Attempt %d/%d establishing SSH over bastion to %s", attempt, sshDialAttempts, vmPrivateIP)

		tunnel, err := bastion.NewTunnelSession(ctx, vmPrivateIP, 22)
		if err != nil {
			lastErr = err
			logging.Logf(ctx, "Attempt %d/%d failed to create bastion tunnel: %v", attempt, sshDialAttempts, err)
			continue
		}

		_ = tunnel.SetDeadline(time.Now().Add(sshDialTimeout))
		stop := context.AfterFunc(ctx, func() { _ = tunnel.Conn.Close() })
		sshConn, chans, reqs, err := ssh.NewClientConn(
			tunnel,
			vmPrivateIP,
			sshConfig,
		)
		stop()
		if ctx.Err() != nil {
			_ = tunnel.Close()
			return nil, ctx.Err()
		}
		if err != nil {
			lastErr = err
			logging.Logf(ctx, "Attempt %d/%d SSH handshake failed: %v", attempt, sshDialAttempts, err)
			_ = tunnel.Close()
			continue
		}
		_ = tunnel.SetDeadline(time.Time{})
		return newSSHClient(ssh.NewClient(sshConn, chans, reqs)), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to establish SSH connection over bastion")
	}
	return nil, lastErr
}
