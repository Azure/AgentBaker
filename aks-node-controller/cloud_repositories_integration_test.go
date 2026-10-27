package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCloudUbuntuRepositoryInitializationIndependentOfOptIn(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	serveCloudTestResponses(t, c, nil, `{"IsOptedInForRootCerts":false}`)
	require.NoError(t, c.writeFile("/etc/apt/sources.list", []byte("original ubuntu sources")))
	require.NoError(t, c.writeFile(defaultAptSourcesDir+"/original.sources", []byte("original extra sources")))
	var commands []string
	run := c.run
	c.run = func(cmd *exec.Cmd) error {
		commands = append(commands, strings.Join(cmd.Args, " "))
		return run(cmd)
	}
	require.NoError(t, c.initialize(t.Context(), cloudInitConfig{
		Location: "westus2", RepoDepotEndpoint: c.wireEndpoint + "/ubuntu/", Timeout: time.Minute,
	}))
	require.Equal(t, []string{"gpg --dearmor", "gpg --dearmor", "apt-get update"}, commands)
	backup, err := os.ReadFile(c.path("/etc/apt/backup/sources.list"))
	require.NoError(t, err)
	require.Equal(t, "original ubuntu sources", string(backup))
	backup, err = os.ReadFile(c.path("/etc/apt/backup/original.sources"))
	require.NoError(t, err)
	require.Equal(t, "original extra sources", string(backup))
	source, err := os.ReadFile(c.path(defaultAptSourcesDir + "/ubuntu.sources"))
	require.NoError(t, err)
	require.Contains(t, string(source), "URIs: "+c.wireEndpoint+"/ubuntu\n")
	require.Contains(t, string(source), "Suites: jammy jammy-updates jammy-backports jammy-security")
	for _, name := range []string{"microsoft-prod", "microsoft-prod-testing"} {
		source, err := os.ReadFile(c.path(defaultAptSourcesDir + "/" + name + ".sources"))
		require.NoError(t, err)
		require.Contains(t, string(source), c.wireEndpoint+"/microsoft/ubuntu/22.04/prod")
		require.Contains(t, string(source), "Architectures: "+runtime.GOARCH)
		require.Contains(t, string(source), "Signed-By: /etc/apt/keyrings/microsoft.asc.gpg /etc/apt/keyrings/msopentech.asc.gpg")
	}
	for _, name := range []string{"microsoft.asc", "msopentech.asc"} {
		key, err := os.ReadFile(c.path("/etc/apt/keyrings/" + name + ".gpg"))
		require.NoError(t, err)
		require.Equal(t, "dearmored-key", string(key))
	}
}

func TestCloudRPMRepositoryInitializationIndependentOfOptIn(t *testing.T) {
	for _, id := range []string{"mariner", osReleaseIDAzureLinux} {
		t.Run(id, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID="+id+"\n")
			serveCloudTestResponses(t, c, nil, `{"IsOptedInForRootCerts":false}`)
			require.NoError(t, c.writeFile(defaultYumReposDir+"/keep.repo", []byte("keep this repository")))
			require.NoError(t, c.writeFile(defaultYumReposDir+"/azurelinux-old.repo", []byte("obsolete")))
			require.NoError(t, c.writeFile(defaultYumReposDir+"/mariner-extras.repo",
				[]byte("[mariner-extras]\nname=Mariner Extras\nbaseurl=https://packages.microsoft.com/cbl-mariner/$releasever/extras/$basearch\n")))
			var commands []string
			c.run = func(cmd *exec.Cmd) error {
				commands = append(commands, strings.Join(cmd.Args, " "))
				return nil
			}
			require.NoError(t, c.initialize(t.Context(), cloudInitConfig{
				Location: "westus2", RepoDepotEndpoint: "https://repo.example/ubuntu", Timeout: time.Minute,
			}))
			require.Equal(t, []string{"dnf makecache -y"}, commands)
			if id == "mariner" {
				for _, repo := range []string{"extras", "extended", "nvidia", "cloud-native"} {
					source, err := os.ReadFile(c.path(defaultYumReposDir + "/mariner-" + repo + ".repo"))
					require.NoError(t, err)
					require.Contains(t, string(source), "https://repo.example/mariner/packages.microsoft.com/")
					require.Contains(t, string(source), "/"+repo+"/")
				}
			} else {
				_, err := os.Stat(c.path(defaultYumReposDir + "/azurelinux-old.repo"))
				require.ErrorIs(t, err, os.ErrNotExist)
				for _, repo := range []string{"amd", "base", "cloud-native", "extended", "ms-non-oss", "ms-oss", "nvidia"} {
					source, err := os.ReadFile(c.path(defaultYumReposDir + "/azurelinux-" + repo + ".repo"))
					require.NoError(t, err)
					require.Contains(t, string(source), "https://repo.example/azurelinux/$releasever/prod/"+repo+"/$basearch")
					require.Contains(t, string(source), "gpgcheck=1\nrepo_gpgcheck=1")
				}
			}
			keep, err := os.ReadFile(c.path(defaultYumReposDir + "/keep.repo"))
			require.NoError(t, err)
			require.Equal(t, "keep this repository", string(keep))
		})
	}
}

func TestCloudRepositorySetupFailureStillStopsInitialization(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	serveCloudTestResponses(t, c, nil, `{"IsOptedInForRootCerts":false}`)
	c.run = func(*exec.Cmd) error {
		t.Fatal("an invalid endpoint must not reach package cache refresh")
		return nil
	}
	err := c.initialize(t.Context(), cloudInitConfig{
		Location: "westus2", RepoDepotEndpoint: "not-a-url", Timeout: time.Minute,
	})
	require.ErrorContains(t, err, "invalid RepoDepot endpoint")
}
