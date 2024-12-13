package ado

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/env"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/release"
)

type Client struct {
	pipelines pipelines.Client
	releases  release.Client
	builds    build.Client
	basicAuth string
}

func NewClient(ctx context.Context) (*Client, error) {
	if env.Vars.ADOPAT == "" {
		return nil, fmt.Errorf("cannot construct ADO client: ADO_PAT missing from environment")
	}

	conn := azuredevops.NewPatConnection("https://dev.azure.com/msazure", env.Vars.ADOPAT)
	pipelinesClient := pipelines.NewClient(ctx, conn)

	releaseClient, err := release.NewClient(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("constructing new ADO release client: %w", err)
	}

	buildsClient, err := build.NewClient(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("constructing new ADO build client: %w", err)
	}

	return &Client{
		pipelines: pipelinesClient,
		releases:  releaseClient,
		builds:    buildsClient,
		basicAuth: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(":%s", env.Vars.ADOPAT))),
	}, nil
}

func (c *Client) BuildEV2Artifacts(ctx context.Context, vhdBuildID string, pollConfig *PollingConfig) (*ArtifactBuild, error) {
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

	build := &ArtifactBuild{
		ID:   *run.Id,
		Name: *run.Name,
	}
	log.Printf("EV2 artifact build started; ID: %d, URL: %s", build.ID, build.URL())
	log.Printf("will poll build status every %s", pollConfig.getBuildCompletionPollInterval())

	ticker := time.NewTicker(pollConfig.getBuildCompletionPollInterval())
	for !isTerminal(*run.State) {
		select {
		case <-ticker.C:
			run, err = c.pipelines.GetRun(ctx, pipelines.GetRunArgs{
				Project:    to.Ptr(projectName),
				PipelineId: to.Ptr(sigReleaseArtifactBuildPipelineID),
				RunId:      &build.ID,
			})
			if err != nil {
				return nil, fmt.Errorf("getting EV2 artifact build pipeline run: %w", err)
			}
			log.Printf("EV2 artifact build %d is in state %q", build.ID, *run.State)
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for EV2 artifact build to finish: %w", ctx.Err())
		}
	}

	log.Printf("SIG release EV2 artifact build %d finished with result: %s", build.ID, *run.State)
	return build, nil
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
	releaseURL, err := extractURL(sigRelease.Links)
	if err != nil {
		return fmt.Errorf("extracting release URL: %w", err)
	}
	log.Printf("created SIG release: %s", releaseURL)
	return nil
}

func (c *Client) GetLatestVHDBuildFromBranch(ctx context.Context, branchName string) (*build.Build, string, error) {
	resp, err := c.builds.GetBuilds(ctx, build.GetBuildsArgs{
		Project:     to.Ptr(projectName),
		Definitions: &[]int{testVHDBuildPipelineDefinitionID},
		BranchName:  to.Ptr(plumbing.NewBranchReferenceName(branchName).String()),
		MinTime: &azuredevops.Time{
			Time: time.Now().Add(-24 * time.Hour),
		},
		MaxTime: &azuredevops.Time{
			Time: time.Now(),
		},
		StatusFilter: &build.BuildStatusValues.Completed,
		// ResultFilter: &build.BuildResultValues.Succeeded, // would not pick up builds that fail abe2es
		QueryOrder: &build.BuildQueryOrderValues.FinishTimeDescending,
	})
	if err != nil {
		return nil, "", fmt.Errorf("getting ADO build list: %w", err)
	}
	if len(resp.Value) < 1 {
		return nil, "", &ErrNoBuildsFound{
			DefinitionID: officialVHDBuildPipelineDefinitionID,
			BranchName:   branchName,
		}
	}
	vhdBuild := &resp.Value[0]
	url, err := extractURL(vhdBuild.Links)
	if err != nil {
		return nil, "", fmt.Errorf("extracting build URL: %w", err)
	}
	return vhdBuild, url, nil
}

func (c *Client) GetImageVersionForVHDBuild(ctx context.Context, vhdBuildID int) (string, error) {
	artifacts, err := c.builds.GetArtifacts(ctx, build.GetArtifactsArgs{
		Project: to.Ptr(projectName),
		BuildId: &vhdBuildID,
	})
	if err != nil {
		return "", fmt.Errorf("getting build artifacts from VHD build %d: %w", vhdBuildID, err)
	}

	var pinfoArtifact *build.BuildArtifact
	for _, artifact := range *artifacts {
		if strings.HasPrefix(*artifact.Name, "publishing-info") {
			pinfoArtifact = &artifact
			break
		}
	}
	if pinfoArtifact == nil {
		return "", fmt.Errorf("unable to find publishing-info artifact from VHD build %d, cannot extract image version", vhdBuildID)
	}

	downloadURL := *pinfoArtifact.Resource.DownloadUrl
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("unable to create new HTTP request: %w", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", c.basicAuth))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("performing GET to download artifact from %s: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("downloading artifact from %s, received HTTP status code: %d", downloadURL, resp.StatusCode)
	}

	var b bytes.Buffer
	bufWriter := bufio.NewWriter(&b)
	size, err := io.Copy(bufWriter, resp.Body)
	if err != nil {
		return "", fmt.Errorf("copying zip contents to buffer: %w", err)
	}

	bufReader := bytes.NewReader(b.Bytes())
	zipReader, err := zip.NewReader(bufReader, size)
	if err != nil {
		return "", fmt.Errorf("creating new zip reader from buffer: %w", err)
	}

	return extractPublishingInfoFromZip(zipReader)
}
