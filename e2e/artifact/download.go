package artifact

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/Azure/agentbakere2e/suite"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	ado "github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	adobuild "github.com/microsoft/azure-devops-go-api/azuredevops/v7/build"
	"go.uber.org/multierr"
)

// NewDownloader constructs a new ADO artifact downloader using the PAT within the specified suite config.
func NewDownloader(ctx context.Context, suiteConfig *suite.Config) (*Downloader, error) {
	conn := ado.NewPatConnection(azureADOOrganizationURL, suiteConfig.PAT)

	buildClient, err := adobuild.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &Downloader{
		basicAuth:   base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(":%s", suiteConfig.PAT))),
		buildClient: buildClient,
	}, nil
}

// DownloadVHDBuildPublishingInfo will download a set of specified VHD publishing info artifacts from a particular VHD build
// and place them in the target directory for further consumption.
func (d *Downloader) DownloadVHDBuildPublishingInfo(ctx context.Context, opts PublishingInfoDownloadOpts) error {
	if err := os.MkdirAll(opts.TargetDir, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create publishing info dir %s: %w", opts.TargetDir, err)
	}

	tempDir, err := os.MkdirTemp("", "publishinginfo")
	defer os.RemoveAll(tempDir)
	if err != nil {
		return fmt.Errorf("unable to create temp directory to store zip archives: %w", err)
	}

	artifactNames, err := d.getVHDPublishingInfoArtifactNames(ctx, opts)
	if err != nil {
		return fmt.Errorf("unable to get VHD publishing info artifact names for VHD build %d: %w", opts.BuildID, err)
	}

	d.errChan = make(chan error)
	d.doneChan = make(chan struct{})

	for _, name := range artifactNames {
		go d.downloadPublishingInfo(ctx, tempDir, name, opts)
	}

	var errs []error
	for i := 0; i < len(artifactNames); i++ {
		select {
		case err := <-d.errChan:
			errs = append(errs, err)
		case <-d.doneChan:
			continue
		}
	}

	return multierr.Combine(errs...)
}

func (d *Downloader) downloadPublishingInfo(ctx context.Context, tempDir, artifactName string, opts PublishingInfoDownloadOpts) {
	defer func() { d.doneChan <- struct{}{} }()
	artifact, err := d.buildClient.GetArtifact(ctx, adobuild.GetArtifactArgs{
		Project:      to.Ptr(cloudNativeComputeProject),
		BuildId:      to.Ptr(opts.BuildID),
		ArtifactName: to.Ptr(artifactName),
	})
	if err != nil {
		if isMissingArtifactError(err) {
			log.Printf("unable to download publishing info %q, not found for build ID: %d", artifactName, opts.BuildID)
		} else {
			d.errChan <- fmt.Errorf("unable get artifact info for %q: %w", artifactName, err)
		}
		return
	}

	downloadURL := *artifact.Resource.DownloadUrl
	zipName := path.Join(tempDir, fmt.Sprintf("%s.zip", artifactName))

	zipOut, err := os.Create(zipName)
	if err != nil {
		d.errChan <- fmt.Errorf("unable to create zip archive %s: %w", zipName, err)
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		d.errChan <- fmt.Errorf("unable to create new HTTP request: %w", err)
		return
	}

	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", d.basicAuth))

	resp, err := client.Do(req)
	if err != nil {
		d.errChan <- fmt.Errorf("unable to perform HTTP request to download artifact: %w", err)
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		d.errChan <- fmt.Errorf("unable to download artifact from %s, received HTTP status code: %d", downloadURL, resp.StatusCode)
		return
	}

	if _, err = io.Copy(zipOut, resp.Body); err != nil {
		d.errChan <- fmt.Errorf("unable to copy artifact data to zip archive: %w", err)
		return
	}

	if err = extractPublishingInfoFromZip(artifactName, zipName, opts.TargetDir); err != nil {
		d.errChan <- fmt.Errorf("unable to extract publishing info from zip archive %s: %w", zipName, err)
		return
	}
}

// getVHDPublishingInfoArtifactNames resolves the set of artifact names which will be downloaded from ADO.
// If opts specifies at least one artifact name, only those will be downloaded. Otherwise, all publishing info artifacts
// from the specified build will be downloaded.
func (d *Downloader) getVHDPublishingInfoArtifactNames(ctx context.Context, opts PublishingInfoDownloadOpts) ([]string, error) {
	var artifactNames []string

	if len(opts.ArtifactNames) > 0 {
		for name := range opts.ArtifactNames {
			artifactNames = append(artifactNames, fmt.Sprintf("publishing-info-%s", name))
		}
		return artifactNames, nil
	}

	artifacts, err := d.buildClient.GetArtifacts(ctx, adobuild.GetArtifactsArgs{
		Project: to.Ptr(cloudNativeComputeProject),
		BuildId: to.Ptr(opts.BuildID),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list build artifacts for VHD build %d: %w", opts.BuildID, err)
	}

	for _, artifact := range *artifacts {
		if strings.HasPrefix(*artifact.Name, "publishing-info") {
			artifactNames = append(artifactNames, *artifact.Name)
		}
	}

	return artifactNames, nil
}

func extractPublishingInfoFromZip(artifactName, zipName, targetDir string) error {
	archive, err := zip.OpenReader(zipName)
	if err != nil {
		return fmt.Errorf("unable to open zip reader for %s: %w", zipName, err)
	}
	defer archive.Close()

	for _, f := range archive.File {
		if strings.HasSuffix(f.Name, publishingInfoJSONFileName) {
			dest := path.Join(targetDir, fmt.Sprintf("publishing-info-%s.json", artifactName))

			destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
			if err != nil {
				return fmt.Errorf("unable to open dest file %s for copying from zip archive %s: %w", dest, zipName, err)
			}
			defer destFile.Close()

			pinfo, err := f.Open()
			if err != nil {
				return fmt.Errorf("unable to open file %s within zip archive %s: %w", f.Name, zipName, err)
			}
			defer pinfo.Close()

			if _, err = io.Copy(destFile, pinfo); err != nil {
				return fmt.Errorf("unable to copy %s from zip archive %s to destination %s: %w", f.Name, zipName, dest, err)
			}

			return nil
		}
	}

	return fmt.Errorf("unable to find %s within zip archive %s", publishingInfoJSONFileName, zipName)
}

func isMissingArtifactError(err error) bool {
	// there doesn't seem to be a proper error type for this,
	// thus we need to assert on thecontents of the error msg itself
	return err != nil && (strings.Contains(err.Error(), "404 Not Found") || strings.Contains(err.Error(), "was not found for build"))
}
