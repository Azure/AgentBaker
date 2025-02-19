package ado

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
	"github.com/stretchr/testify/assert"
)

type fakePipelinesClient struct {
	pipelines.Client
	runPipeline func(context.Context, pipelines.RunPipelineArgs) (*pipelines.Run, error)
	getRun      func(context.Context, pipelines.GetRunArgs) (*pipelines.Run, error)
}

func (c *fakePipelinesClient) RunPipeline(ctx context.Context, args pipelines.RunPipelineArgs) (*pipelines.Run, error) {
	return c.runPipeline(ctx, args)
}

func (c *fakePipelinesClient) GetRun(ctx context.Context, args pipelines.GetRunArgs) (*pipelines.Run, error) {
	return c.getRun(ctx, args)
}

type fakeReleaseClient struct {
	release.Client
	createRelease func(context.Context, release.CreateReleaseArgs) (*release.Release, error)
}

func (c *fakeReleaseClient) CreateRelease(ctx context.Context, args release.CreateReleaseArgs) (*release.Release, error) {
	return c.createRelease(ctx, args)
}

func TestBuildEV2Artifacts(t *testing.T) {
	cases := []struct {
		name            string
		timeout         *time.Duration
		pollConfig      *PollingConfig
		pipelinesClient pipelines.Client
		expected        *ArtifactBuild
		wantErr         string
	}{
		{
			name: "it should return an error a new build pipeline cannot be executed",
			pipelinesClient: &fakePipelinesClient{
				runPipeline: func(ctx context.Context, args pipelines.RunPipelineArgs) (*pipelines.Run, error) {
					expectCorrectRunPipelineArgs(t, args)
					return nil, fmt.Errorf("unable to start new pipeline run")
				},
			},
			expected: nil,
			wantErr:  "running EV2 artifact build pipeline: unable to start new pipeline run",
		},
		{
			name: "it should return an error if the initial pipeline state is terminal",
			pipelinesClient: &fakePipelinesClient{
				runPipeline: func(ctx context.Context, args pipelines.RunPipelineArgs) (*pipelines.Run, error) {
					expectCorrectRunPipelineArgs(t, args)
					return &pipelines.Run{
						Id:    to.Ptr(1),
						Name:  to.Ptr("run"),
						State: &pipelines.RunStateValues.Unknown,
					}, nil
				},
			},
			expected: nil,
			wantErr:  "new pipeline run 1 has unexpected state: unknown",
		},
		{
			name:    "it should return an error if the context is cancelled",
			timeout: to.Ptr(50 * time.Millisecond),
			pipelinesClient: &fakePipelinesClient{
				runPipeline: func(ctx context.Context, args pipelines.RunPipelineArgs) (*pipelines.Run, error) {
					expectCorrectRunPipelineArgs(t, args)
					return &pipelines.Run{
						Id:    to.Ptr(1),
						Name:  to.Ptr("run"),
						State: &pipelines.RunStateValues.InProgress,
					}, nil
				},
				getRun: func(ctx context.Context, args pipelines.GetRunArgs) (*pipelines.Run, error) {
					expectCorrectGetRunArgs(t, args)
					return &pipelines.Run{
						Id:    to.Ptr(1),
						Name:  to.Ptr("run"),
						State: &pipelines.RunStateValues.InProgress,
					}, nil
				},
			},
		},
		{
			name: "it should return an error if unable to poll pipeline run status",
			pipelinesClient: &fakePipelinesClient{
				runPipeline: func(ctx context.Context, args pipelines.RunPipelineArgs) (*pipelines.Run, error) {
					expectCorrectRunPipelineArgs(t, args)
					return &pipelines.Run{
						Id:    to.Ptr(1),
						Name:  to.Ptr("run"),
						State: &pipelines.RunStateValues.InProgress,
					}, nil
				},
				getRun: func(ctx context.Context, args pipelines.GetRunArgs) (*pipelines.Run, error) {
					expectCorrectGetRunArgs(t, args)
					return nil, fmt.Errorf("unable to get run")
				},
			},
			pollConfig: &PollingConfig{
				BuildCompletionPollInterval: to.Ptr(250 * time.Millisecond),
			},
			expected: nil,
			wantErr:  "getting EV2 artifact build pipeline run: unable to get run",
		},
		{
			name: "it should return a valid result when no errors occur",
			pipelinesClient: &fakePipelinesClient{
				runPipeline: func(ctx context.Context, args pipelines.RunPipelineArgs) (*pipelines.Run, error) {
					expectCorrectRunPipelineArgs(t, args)
					return &pipelines.Run{
						Id:    to.Ptr(1),
						Name:  to.Ptr("run"),
						State: &pipelines.RunStateValues.InProgress,
					}, nil
				},
				getRun: func(ctx context.Context, args pipelines.GetRunArgs) (*pipelines.Run, error) {
					expectCorrectGetRunArgs(t, args)
					return &pipelines.Run{
						Id:    to.Ptr(1),
						Name:  to.Ptr("run"),
						State: &pipelines.RunStateValues.Completed,
					}, nil
				},
			},
			pollConfig: &PollingConfig{
				BuildCompletionPollInterval: to.Ptr(250 * time.Millisecond),
			},
			expected: &ArtifactBuild{
				ID:   1,
				Name: "run",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.timeout != nil {
				withTimeout, cancel := context.WithTimeout(ctx, *c.timeout)
				defer cancel()
				ctx = withTimeout
			}

			client := &Client{
				pipelines: c.pipelinesClient,
			}
			actualBuild, err := client.BuildEV2Artifacts(ctx, "1", c.pollConfig)
			if c.wantErr != "" {
				assert.EqualError(t, err, c.wantErr)
			}
			assert.Equal(t, c.expected, actualBuild)
		})
	}
}

func TestCreateSIGRelease(t *testing.T) {
	cases := []struct {
		name          string
		releaseClient release.Client
		wantErr       string
	}{
		{
			name: "it should return an error when unable to create a release via the release client",
			releaseClient: &fakeReleaseClient{
				createRelease: func(ctx context.Context, args release.CreateReleaseArgs) (*release.Release, error) {
					expectCorrectCreateReleaseArgs(t, args)
					return nil, fmt.Errorf("unable to create release")
				},
			},
			wantErr: "creating SIG release: unable to create release",
		},
		{
			name: "it should return an error if unable to extract the release URL",
			releaseClient: &fakeReleaseClient{
				createRelease: func(ctx context.Context, args release.CreateReleaseArgs) (*release.Release, error) {
					expectCorrectCreateReleaseArgs(t, args)
					return &release.Release{
						Links: map[string]interface{}{
							"web": map[string]interface{}{
								"bad": "url",
							},
						},
					}, nil
				},
			},
			wantErr: "extracting release URL: failed to find href key in release web links",
		},
		{
			name: "it should create the release when no errors occur",
			releaseClient: &fakeReleaseClient{
				createRelease: func(ctx context.Context, args release.CreateReleaseArgs) (*release.Release, error) {
					expectCorrectCreateReleaseArgs(t, args)
					return &release.Release{
						Links: map[string]interface{}{
							"web": map[string]interface{}{
								"href": "url",
							},
						},
					}, nil
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()

			client := &Client{
				releases: c.releaseClient,
			}
			err := client.CreateSIGRelease(ctx, &ArtifactBuild{
				ID:   1,
				Name: "build",
			})
			if c.wantErr != "" {
				assert.EqualError(t, err, c.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestArtifactBuild(t *testing.T) {
	cases := []struct {
		name     string
		b        *ArtifactBuild
		expected string
	}{
		{
			name:     "build URL when build ID is unset should be empty",
			b:        &ArtifactBuild{},
			expected: "",
		},
		{
			name:     "build URL should be valid when build ID is set",
			b:        &ArtifactBuild{ID: 1234567},
			expected: "https://msazure.visualstudio.com/CloudNativeCompute/_build/results?buildId=1234567&view=results",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, c.b.URL())
		})
	}
}

func expectCorrectRunPipelineArgs(t *testing.T, actual pipelines.RunPipelineArgs) {
	expected := pipelines.RunPipelineArgs{
		Project:    to.Ptr(projectName),
		PipelineId: to.Ptr(sigReleaseArtifactBuildPipelineID),
		RunParameters: &pipelines.RunPipelineParameters{
			Variables: to.Ptr(getArtifactBuildVariables("1")),
		},
	}
	assert.Equal(t, expected, actual)
}

func expectCorrectGetRunArgs(t *testing.T, actual pipelines.GetRunArgs) {
	expected := pipelines.GetRunArgs{
		Project:    to.Ptr(projectName),
		PipelineId: to.Ptr(sigReleaseArtifactBuildPipelineID),
		RunId:      to.Ptr(1),
	}
	assert.Equal(t, expected, actual)
}

func expectCorrectCreateReleaseArgs(t *testing.T, actual release.CreateReleaseArgs) {
	expected := release.CreateReleaseArgs{
		Project: to.Ptr(projectName),
		ReleaseStartMetadata: &release.ReleaseStartMetadata{
			DefinitionId: to.Ptr(sigReleaseDefinitionID),
			Artifacts: &[]release.ArtifactMetadata{
				{
					Alias: to.Ptr(sigReleaseArtifactsAlias),
					InstanceReference: &release.BuildVersion{
						Id:   to.Ptr("1"),
						Name: to.Ptr("build"),
					},
				},
			},
		},
	}
	assert.Equal(t, expected, actual)
}
