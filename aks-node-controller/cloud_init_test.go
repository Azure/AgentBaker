package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/require"
)

func TestCloudInitializationCertificateModes(t *testing.T) {
	cert := cloudTestCertificate(t)
	for _, osCase := range []struct{ name, release, trustDir, update string }{
		{"Ubuntu", cloudTestUbuntu, "/usr/local/share/ca-certificates", "update-ca-certificates"},
		{"Mariner", "ID=mariner\n", "/etc/pki/ca-trust/source/anchors", "update-ca-trust"},
		{"AzureLinux", "ID=azurelinux\n", "/etc/pki/ca-trust/source/anchors", "update-ca-trust"},
		{"Flatcar", "ID=flatcar\n", "/etc/ssl/certs", "update-ca-certificates"},
		{"ACL", "ID=azurecontainerlinux\n", "/etc/pki/ca-trust/source/anchors", "update-ca-trust"},
	} {
		for _, mode := range []struct {
			name, location, optIn string
			eligible              bool
		}{
			{"not opted in", "westus2", `{"IsOptedInForRootCerts":false}`, false},
			{"rcv1p", "westus2", `{"IsOptedInForRootCerts":true}`, true},
			{"legacy", " US NAT East ", "not used", true},
		} {
			t.Run(osCase.name+"/"+mode.name, func(t *testing.T) {
				c := newTestCloudInitializer(t, osCase.release)
				requests := serveCloudTestResponses(t, c, cert, mode.optIn)
				var commands []string
				run := c.run
				c.run = func(cmd *exec.Cmd) error {
					commands = append(commands, strings.Join(cmd.Args, " "))
					return run(cmd)
				}
				err := c.initialize(t.Context(), cloudInitConfig{Location: mode.location, Timeout: time.Minute})
				require.NoError(t, err)
				if !mode.eligible {
					require.Empty(t, commands)
					require.Len(t, requests, 1)
					return
				}
				require.Equal(t, osCase.update, commands[0])
				filename := "root.crt"
				if osCase.name == "Flatcar" {
					filename = "root.pem"
				}
				got, err := os.ReadFile(c.path(filepath.Join(osCase.trustDir, filename)))
				require.NoError(t, err)
				want := cert
				if mode.name == "legacy" {
					want = []byte(strings.TrimSuffix(string(cert), "\n") + "\r\n")
				}
				require.Equal(t, want, got)
				assertCloudSchedule(t, c, osCase.name, commands)
				if mode.name == "legacy" {
					require.Len(t, requests, 1)
				} else {
					require.Len(t, requests, 5)
				}
			})
		}
	}
}

func TestCloudCertificateOptInNonTrueResponsesSkip(t *testing.T) {
	for _, response := range []string{
		"not json", " ", `{}`, `null`, `[]`, `{"IsOptedInForRootCerts":false}`,
		`{"IsOptedInForRootCerts":null}`, `{"IsOptedInForRootCerts":"false"}`,
		`{"IsOptedInForRootCerts":"true"}`, `{"IsOptedInForRootCerts":1}`,
		`{"isOptedInForRootCerts":true}`,
	} {
		t.Run(response, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			requests := serveCloudTestResponses(t, c, nil, response)
			c.run = func(*exec.Cmd) error {
				t.Fatal("must not update trust or schedules without explicit opt-in")
				return nil
			}
			require.NoError(t, c.initialize(t.Context(), cloudInitConfig{Location: "westus2", Timeout: time.Minute}))
			require.Len(t, requests, 1)
			events := c.eventLogger.Events()
			require.NotEmpty(t, events)
			require.Equal(t, string(helpers.EventLevelInformational), events[len(events)-1].EventLevel)
		})
	}
}

func TestCloudLegacyCertificateResponseBehavior(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		wantError      bool
	}{
		{"empty array", `{"Certificates":[]}`, false},
		{"non-crt file", `{"Certificates":[{"Name":"root.txt","CertBody":"payload"}]}`, false},
		{"empty body", `{"Certificates":[{"Name":"root.cer","CertBody":""}]}`, false},
		{"trust update failure", `{"Certificates":[{"Name":"root.cer","CertBody":"certificate payload"}]}`, false},
		{"invalid name", `{"Certificates":[{"Name":"../root.crt"}]}`, false},
		{"non-string name", `{"Certificates":[{"Name":123}]}`, false},
		{"missing array", `{}`, true},
		{"null array", `{"Certificates":null}`, true},
		{"missing body", `{"Certificates":[{"Name":"root.cer"}]}`, true},
		{"null body", `{"Certificates":[{"Name":"root.cer","CertBody":null}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.response)
			}))
			defer server.Close()
			c.wireEndpoint = server.URL
			var commands []string
			c.run = func(cmd *exec.Cmd) error {
				commands = append(commands, cmd.Args[0])
				if cmd.Args[0] == "update-ca-trust" {
					return errors.New("trust store update failed")
				}
				return nil
			}
			err := c.initialize(t.Context(), cloudInitConfig{Location: "usnateast", Timeout: time.Minute})
			if tc.wantError {
				require.Error(t, err)
				require.Empty(t, commands)
			} else {
				require.NoError(t, err)
				require.Contains(t, commands, "crontab")
			}
		})
	}
}

func TestCloudLegacyCertificateCacheWriteFailureStopsInitialization(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	serveCloudTestResponses(t, c, cloudTestCertificate(t), "not used")
	require.NoError(t, c.writeFile("/root/AzureCACertificates/root.crt/keep", []byte("keep")))
	c.run = func(*exec.Cmd) error {
		t.Fatal("must not install certificates or schedules after a cache write failure")
		return nil
	}
	err := c.initialize(t.Context(), cloudInitConfig{Location: "usnateast", Timeout: time.Minute})
	require.ErrorContains(t, err, "cache legacy Azure cloud certificate")
}

func TestCloudInitializationHonorsConfiguredTimeout(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	c.wireEndpoint = server.URL
	start := time.Now()
	err := c.initialize(t.Context(), cloudInitConfig{Location: "westus2", Timeout: 25 * time.Millisecond})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), time.Second)
}

func TestCloudInitializationStopsOnTrustAndScheduleFailures(t *testing.T) {
	for _, tc := range []struct {
		name, location, failedCommand, wantError string
	}{
		{"trust store", "westus2", "update-ca-trust", "host operation failed"},
		{"missing location", " \t", "", "requires a location"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID=azurelinux\n")
			serveCloudTestResponses(t, c, cloudTestCertificate(t), `{"IsOptedInForRootCerts":true}`)
			c.run = func(cmd *exec.Cmd) error {
				if cmd.Args[0] == tc.failedCommand {
					return errors.New("host operation failed")
				}
				require.NotEqual(t, "dnf", cmd.Args[0])
				if tc.failedCommand == "" {
					require.NotEqual(t, "crontab", cmd.Args[0])
				}
				return nil
			}
			err := c.initialize(t.Context(), cloudInitConfig{
				Location: tc.location, RepoDepotEndpoint: "https://repo.example/ubuntu", Timeout: time.Minute,
			})
			require.ErrorContains(t, err, tc.wantError)
		})
	}
}
