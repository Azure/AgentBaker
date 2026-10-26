package main

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudRepositoriesSkipEmptyEndpointAndImmutableOS(t *testing.T) {
	for _, tc := range []struct{ id, endpoint string }{
		{osReleaseIDUbuntu, ""}, {"mariner", ""}, {osReleaseIDAzureLinux, ""},
		{osReleaseIDFlatcar, "https://repo.example"}, {osReleaseIDAzureContainerLinux, "https://repo.example"},
	} {
		t.Run(tc.id+"/"+tc.endpoint, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID="+tc.id+"\n")
			c.run = func(*exec.Cmd) error {
				t.Fatal("no repository commands expected")
				return nil
			}
			require.NoError(t, c.initializeRepositories(t.Context(), cloudPlatform{ID: tc.id}, tc.endpoint))
		})
	}
}

func TestCloudRepositoryEndpointValidation(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	for _, endpoint := range []string{
		"not-a-url", "file:///etc/apt", "https://repo.example\nSigned-By: bad", "https://user:password@repo.example",
		"https://repo.example?query=1", "https://repo.example/#fragment",
	} {
		require.ErrorContains(t, c.initializeRepositories(t.Context(), cloudPlatform{ID: osReleaseIDUbuntu}, endpoint), "invalid RepoDepot endpoint")
	}
}

func TestCloudRepositoryCacheRetriesAndFailsExplicitly(t *testing.T) {
	for _, name := range []string{"apt-get", "dnf"} {
		for _, recoverAfter := range []int{2, cloudRequestAttempts + 1} {
			t.Run(name+"/"+strconv.Itoa(recoverAfter), func(t *testing.T) {
				c := newTestCloudInitializer(t, cloudTestUbuntu)
				attempts := 0
				c.run = func(cmd *exec.Cmd) error {
					attempts++
					if attempts < recoverAfter {
						if name == "dnf" {
							_, _ = io.WriteString(cmd.Stderr, "Error: repository metadata is unavailable\n")
							return nil
						}
						return errors.New("cache refresh failed")
					}
					return nil
				}
				err := c.updateRepositoryCache(t.Context(), name, "update")
				if recoverAfter > cloudRequestAttempts {
					require.ErrorContains(t, err, "refresh RepoDepot package cache")
					require.ErrorIs(t, err, errCloudRepositoryCacheRefresh)
					require.Equal(t, cloudRequestAttempts, attempts)
				} else {
					require.NoError(t, err)
					require.Equal(t, recoverAfter, attempts)
				}
			})
		}
	}
	t.Run("canceled", func(t *testing.T) {
		c := newTestCloudInitializer(t, cloudTestUbuntu)
		c.run = func(*exec.Cmd) error {
			t.Fatal("canceled context must not execute a command")
			return nil
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := c.updateRepositoryCache(ctx, "apt-get", "update")
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, errCloudRepositoryCacheRefresh)
	})
}
