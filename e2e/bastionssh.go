package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
}

func (b *Bastion) NewTunnelSession(targetHost string, port uint16) (*tunnelSession, error) {
	session, err := b.newSessionToken(targetHost, port)
	if err != nil {
		return nil, err
	}

	wsUrl := fmt.Sprintf("wss://%v/webtunnelv2/%v?X-Node-Id=%v", b.dnsName, session.WebsocketToken, session.NodeID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	ws, _, err := websocket.Dial(ctx, wsUrl, &websocket.DialOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	cancel()
	if err != nil {
		return nil, err
	}

	ws.SetReadLimit(32 * 1024 * 1024)

	return &tunnelSession{
		bastion: b,
		ws:      ws,
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

func (t *tunnelSession) Pipe(conn net.Conn) error {

	defer t.Close()
	defer conn.Close()

	done := make(chan error, 2)

	go func() {
		for {
			_, data, err := t.ws.Read(context.Background())
			if err != nil {
				done <- err
				return
			}

			if _, err := io.Copy(conn, bytes.NewReader(data)); err != nil {
				done <- err
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 4096) // 4096 is copy from az cli bastion code

		for {
			n, err := conn.Read(buf)
			if err != nil {
				done <- err
				return
			}

			if err := t.ws.Write(context.Background(), websocket.MessageBinary, buf[:n]); err != nil {
				done <- err
				return
			}
		}
	}()

	return <-done
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
) (*ssh.Client, error) {

	// Create Bastion tunnel session (SSH = port 22)
	tunnel, err := bastion.NewTunnelSession(
		vmPrivateIP,
		22,
	)
	if err != nil {
		return nil, err
	}

	// Create in-memory connection pair
	sshSide, tunnelSide := net.Pipe()

	// Start Bastion tunnel piping
	go func() {
		_ = tunnel.Pipe(tunnelSide)
		fmt.Printf("Closed tunnel for VM IP %s\n", vmPrivateIP)
	}()

	// SSH client configuration
	sshConfig, err := sshClientConfig("azureuser", sshPrivateKey)
	if err != nil {
		return nil, err
	}

	// Establish SSH over the Bastion tunnel
	sshConn, chans, reqs, err := ssh.NewClientConn(
		sshSide,
		vmPrivateIP,
		sshConfig,
	)
	if err != nil {
		sshSide.Close()
		return nil, err
	}

	return ssh.NewClient(sshConn, chans, reqs), nil
}
