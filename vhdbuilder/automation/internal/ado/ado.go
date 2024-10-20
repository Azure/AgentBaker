package ado

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
)

const (
	sigReleaseArtifactBuildPipelineID = 319030
	sigReleaseDefinitionID            = 494
)

const (
	projectName = "CloudNativeCompute"
)

const (
	sigReleaseArtifactsAlias = "ev2_artifacts"
)

var (
	pollBuildCompletionInterval = 30 * time.Second
)

type Client struct {
	pipelines pipelines.Client
	releases  release.Client
}

func NewClient(ctx context.Context, pat string) (*Client, error) {
	conn := azuredevops.NewPatConnection("https://dev.azure.com/msazure", pat)
	pipelinesClient := pipelines.NewClient(ctx, conn)
	releaseClient, err := release.NewClient(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("constructing new ADO release client: %w", err)
	}
	return &Client{
		pipelines: pipelinesClient,
		releases:  releaseClient,
	}, nil
}

type ArtifactBuild struct {
	ID   int
	Name string
}

func (c *Client) BuildEV2Artifacts(ctx context.Context, vhdBuildID string) (*ArtifactBuild, error) {
	run, err := c.pipelines.RunPipeline(ctx, pipelines.RunPipelineArgs{
		Project:    to.Ptr(projectName),
		PipelineId: to.Ptr(sigReleaseArtifactBuildPipelineID),
		RunParameters: &pipelines.RunPipelineParameters{
			Variables: to.Ptr(getArtifactBuildVariables(vhdBuildID)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("running EV2 artifact build pipeline: %w", err)
	}
	if isTerminal(*run.State) {
		return nil, fmt.Errorf("new pipeline run %d has unexpected state: %s", *run.Id, *run.State)
	}

	buildID := *run.Id
	buildName := *run.Name
	log.Printf("EV2 artifact build started; ID: %d, URL: %s", buildID, getBuildURL(buildID))

	ticker := time.NewTicker(pollBuildCompletionInterval)
	for !isTerminal(*run.State) {
		select {
		case <-ticker.C:
			run, err = c.pipelines.GetRun(ctx, pipelines.GetRunArgs{
				Project:    to.Ptr(projectName),
				PipelineId: to.Ptr(sigReleaseArtifactBuildPipelineID),
				RunId:      &buildID,
			})
			if err != nil {
				return nil, fmt.Errorf("getting SIG release EV2 artifact build pipeline run: %w", err)
			}
			log.Printf("SIG release EV2 artifact build %d is in state %q, will check status again in %s", buildID, *run.State, pollBuildCompletionInterval)
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for EV2 artifact build to finish: %w", ctx.Err())
		}
	}

	log.Printf("SIG release EV2 artifact build %d finished with result: %s", buildID, *run.State)
	return &ArtifactBuild{
		ID:   buildID,
		Name: buildName,
	}, nil
}

func (c *Client) CreateSIGRelease(ctx context.Context, source *ArtifactBuild) error {
	sigRelease, err := c.releases.CreateRelease(ctx, release.CreateReleaseArgs{
		Project: to.Ptr(projectName),
		ReleaseStartMetadata: &release.ReleaseStartMetadata{
			DefinitionId: to.Ptr(sigReleaseDefinitionID),
			Artifacts: &[]release.ArtifactMetadata{
				{
					Alias: to.Ptr(sigReleaseArtifactsAlias),
					InstanceReference: &release.BuildVersion{
						Id:   to.Ptr(fmt.Sprintf("%d", source.ID)),
						Name: to.Ptr(source.Name),
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("creating SIG release: %w", err)
	}
	log.Printf("created SIG release: %d", sigRelease.Id)
	return nil
}

func getArtifactBuildVariables(vhdBuildID string) map[string]pipelines.Variable {
	return map[string]pipelines.Variable{
		"VHD_PIPELINE_RUN_ID": {
			IsSecret: to.Ptr(false),
			Value:    to.Ptr(vhdBuildID),
		},
	}
}

func getBuildURL(buildID int) string {
	return fmt.Sprintf("https://msazure.visualstudio.com/%s/_build/results?buildId=%d&view=results", projectName, buildID)
}

func isTerminal(state pipelines.RunState) bool {
	return state != pipelines.RunStateValues.InProgress && state != "NotStarted"
}
