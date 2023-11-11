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
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	"github.com/microsoft/azure-devops-go-api/azuredevops/build"
	"go.uber.org/multierr"
)

var (
	defaultSKUList = []string{
		"1804-containerd",
		"1804-fips-containerd",
		"1804-fips-gen2-containerd",
		"1804-gen2-containerd",
		"1804-gpu-containerd",
		"2004-fips-containerd",
		"2004-cvm-gen2-containerd",
		"2004-fips-gen2-containerd",
		"2204-arm64-gen2-containerd",
		"2204-containerd",
		"2204-gen2-containerd",
		"2204-tl-gen2-containerd",
		"azurelinuxv2-gen1",
		"azurelinuxv2-gen2-arm64",
		"azurelinuxv2-gen2",
		"azurelinuxv2-gen2-fips",
		"azurelinuxv2-gen2-kata",
		"azurelinuxv2-gen2-trustedlaunch",
		"marinerv2-gen1",
		"marinerv2-gen1-fips",
		"marinerv2-gen2",
		"marinerv2-gen2-arm64",
		"marinerv2-gen2-fips",
		"marinerv2-gen2-kata",
		"marinerv2-gen2-trustedlaunch",
	}
)

type PublishingInfoDownloadOpts struct {
	SKUList   []string
	TargetDir string
	BuildID   int
}

type Downloader struct {
	basicAuth   string
	buildClient build.Client

	errChan  chan error
	doneChan chan struct{}
}

func NewDownloader(ctx context.Context, suiteConfig *suite.Config) (*Downloader, error) {
	conn := azuredevops.NewPatConnection(azureADOOrganizationURL, suiteConfig.PAT)

	buildClient, err := build.NewClient(ctx, conn)
	if err != nil {
		return nil, err
	}

	return &Downloader{
		basicAuth:   base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(":%s", suiteConfig.PAT))),
		buildClient: buildClient,
	}, nil
}

func (d *Downloader) DownloadVHDBuildPublishingInfo(ctx context.Context, opts PublishingInfoDownloadOpts) error {
	if err := os.MkdirAll(opts.TargetDir, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create publishing info dir %s: %w", opts.TargetDir, err)
	}

	tempDir, err := os.MkdirTemp("", "publishinginfo")
	defer os.RemoveAll(tempDir)
	if err != nil {
		return fmt.Errorf("unable to create temp directory to store zip archives: %w", err)
	}

	skuList := defaultSKUList
	if len(opts.SKUList) > 0 {
		skuList = opts.SKUList
	}

	d.errChan = make(chan error)
	d.doneChan = make(chan struct{})

	for _, sku := range skuList {
		go d.downloadPublishingInfo(
			ctx,
			tempDir,
			fmt.Sprintf("publishing-info-%s", sku),
			opts,
		)
	}

	var errs []error
	for i := 0; i < len(skuList); i++ {
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
	artifact, err := d.buildClient.GetArtifact(ctx, build.GetArtifactArgs{
		Project:      to.Ptr(cloudNativeComputeProject),
		BuildId:      to.Ptr(opts.BuildID),
		ArtifactName: to.Ptr(artifactName),
	})
	if err != nil {
		if !isMissingArtifactError(err) {
			d.errChan <- fmt.Errorf("unable get artifact info for %q: %w", artifactName, err)
		} else {
			log.Printf("unable to download publishing info %q, was not found for build ID: %d", artifactName, opts.BuildID)
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

			publishingInfoFile, err := f.Open()
			if err != nil {
				return fmt.Errorf("unable to open file %s within zip archive %s: %w", f.Name, zipName, err)
			}
			defer publishingInfoFile.Close()

			if _, err = io.Copy(destFile, publishingInfoFile); err != nil {
				return fmt.Errorf("unable to copy %s from zip archive %s to destination %s: %w", f.Name, zipName, dest, err)
			}

			return nil
		}
	}

	return fmt.Errorf("unable to find %s within zip archive %s", publishingInfoJSONFileName, zipName)
}

func isMissingArtifactError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "was not found for build")
}
