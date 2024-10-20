package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
)

const (
	envKeyADOPAT     = "ADO_PAT"
	envKeyVHDBuildID = "VHD_BUILD_ID"
)

const (
	sigReleaseEV2ArtifactBuildPipelineID = 319030
	sigReleaseDefinitionID               = 494
)

const (
	sigReleaseEV2ArtifactsAlias = "ev2_artifacts"
)

const (
	cloudNativeComputeProjectName = "CloudNativeCompute"
	adoOrgURL                     = "https://dev.azure.com/msazure"
)

func main() {
	pat := os.Getenv(envKeyADOPAT)
	if pat == "" {
		panic("expected non-empty ADO_PAT")
	}
	vhdBuildID := os.Getenv(envKeyVHDBuildID)
	if vhdBuildID == "" {
		panic("expected non-empty VHD_BUILD_ID")
	}

	conn := azuredevops.NewPatConnection(adoOrgURL, pat)
	ctx := context.Background()

	log.Printf("creating ADO pipeline and release clients...")
	pipelinesClient := pipelines.NewClient(ctx, conn)
	releaseClient, err := release.NewClient(ctx, conn)
	if err != nil {
		panic(err)
	}

	log.Printf("building EV2 artifacts...")
	buildID, buildNumber, err := buildEV2Artifacts(ctx, pipelinesClient, vhdBuildID)
	if err != nil {
		panic(err)
	}

	log.Printf("creating SIG release...")
	if err := createSIGRelease(ctx, releaseClient, buildID, buildNumber); err != nil {
		panic(err)
	}
}

func createSIGRelease(ctx context.Context, releaseClient release.Client, artifactBuildID int, artifactBuildNumber string) error {
	sigRelease, err := releaseClient.CreateRelease(ctx, release.CreateReleaseArgs{
		Project: to.Ptr(cloudNativeComputeProjectName),
		ReleaseStartMetadata: &release.ReleaseStartMetadata{
			DefinitionId: to.Ptr(sigReleaseDefinitionID),
			Artifacts: &[]release.ArtifactMetadata{
				{
					Alias: to.Ptr(sigReleaseEV2ArtifactsAlias),
					InstanceReference: &release.BuildVersion{
						Id:   to.Ptr(fmt.Sprintf("%d", artifactBuildID)),
						Name: to.Ptr(artifactBuildNumber),
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

func buildEV2Artifacts(ctx context.Context, pipelinesClient pipelines.Client, vhdBuildID string) (int, string, error) {
	sleepInterval := 30 * time.Second
	run, err := pipelinesClient.RunPipeline(ctx, pipelines.RunPipelineArgs{
		Project:    to.Ptr(cloudNativeComputeProjectName),
		PipelineId: to.Ptr(sigReleaseEV2ArtifactBuildPipelineID),
		RunParameters: &pipelines.RunPipelineParameters{
			Variables: to.Ptr(getReleaseArtifactRunParameters(vhdBuildID)),
		},
	})
	if err != nil {
		return 0, "", fmt.Errorf("running SIG release EV2 artifact build pipeline: %w", err)
	}
	if *run.State != pipelines.RunStateValues.InProgress && *run.State != "NotStarted" {
		return 0, "", fmt.Errorf("new pipeline run %d has unexpected state: %s", *run.Id, *run.State)
	}

	runID := *run.Id
	runNumber := *run.Name
	log.Printf("SIG release EV2 artifact build ID: %d, waiting %s before checking initial run state...", runID, sleepInterval)

	for {
		time.Sleep(sleepInterval)
		run, err = pipelinesClient.GetRun(ctx, pipelines.GetRunArgs{
			Project:    to.Ptr(cloudNativeComputeProjectName),
			PipelineId: to.Ptr(sigReleaseEV2ArtifactBuildPipelineID),
			RunId:      &runID,
		})
		if err != nil {
			return 0, "", fmt.Errorf("getting SIG release EV2 artifact build pipeline run: %w", err)
		}
		if *run.State != pipelines.RunStateValues.InProgress && *run.State != "NotStarted" {
			break
		}
		log.Printf("SIG release EV2 artifact build %d is in state %q, will check status again in %s", runID, *run.State, sleepInterval)
	}

	log.Printf("SIG release EV2 artifact build %d result: %s", runID, *run.State)
	return runID, runNumber, nil
}

func getReleaseArtifactRunParameters(vhdBuildID string) map[string]pipelines.Variable {
	return map[string]pipelines.Variable{
		"VHD_PIPELINE_RUN_ID": {
			IsSecret: to.Ptr(false),
			Value:    to.Ptr(vhdBuildID),
		},
	}
}
