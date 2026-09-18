package scenario

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/stretchr/testify/require"
)

func TestNativeANCCustomData(t *testing.T) {
	config := &aksnodeconfigv1.Configuration{
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{CoreDnsServiceIp: "172.16.0.53"},
		},
	}
	data, err := nativeANCCustomData(config, "https://example.com/anc")
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(data)
	require.NoError(t, err)
	message, err := mail.ReadMessage(strings.NewReader(string(decoded)))
	require.NoError(t, err)
	media, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", media)
	reader := multipart.NewReader(message.Body, params["boundary"])
	part, err := reader.NextPart()
	require.NoError(t, err)
	require.Equal(t, "text/cloud-boothook", part.Header.Get("Content-Type"))
	content, err := io.ReadAll(part)
	require.NoError(t, err)
	boothook := string(content)
	require.NotContains(t, boothook, "aks-node-controller-nbc-cmd.sh")
	require.Contains(t, boothook, "https://example.com/anc")
	require.Less(t, strings.Index(boothook, "curl -fSL"), strings.Index(boothook, "launching aks-node-controller"))
	_, encoded, found := strings.Cut(boothook, "base64 -d >"+nodeconfigutils.AKSNodeConfigFilePath+"\n")
	require.True(t, found)
	encoded, _, found = strings.Cut(encoded, "\nEOF")
	require.True(t, found)
	jsonConfig, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	roundTrip, err := nodeconfigutils.UnmarshalConfigurationV1(jsonConfig)
	require.NoError(t, err)
	require.Equal(t, "172.16.0.53", roundTrip.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp())
	part, err = reader.NextPart()
	require.NoError(t, err)
	require.Equal(t, "text/cloud-config", part.Header.Get("Content-Type"))
	_, err = reader.NextPart()
	require.ErrorIs(t, err, io.EOF)
}

func TestNativeANCRejectsIncompleteConfig(t *testing.T) {
	_, err := nativeANCCustomData(nil, "https://example.com/anc")
	require.Error(t, err)
	_, err = nativeANCCustomData(&aksnodeconfigv1.Configuration{DisableCustomData: true}, "https://example.com/anc")
	require.Error(t, err)
}
