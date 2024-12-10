package ado

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
)

func getArtifactBuildVariables(vhdBuildID string) map[string]pipelines.Variable {
	return map[string]pipelines.Variable{
		"VHD_PIPELINE_RUN_ID": {
			IsSecret: to.Ptr(false),
			Value:    to.Ptr(vhdBuildID),
		},
	}
}

func isTerminal(state pipelines.RunState) bool {
	return state != pipelines.RunStateValues.InProgress && state != "NotStarted"
}

func extractURL(links interface{}) (string, error) {
	linkMap, ok := links.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unable to convert release links to map[string]interface{}")
	}
	webLinkMap, ok := linkMap["web"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unable to convert release web links to map[string]string")
	}
	if _, ok := webLinkMap["href"]; !ok {
		return "", fmt.Errorf("failed to find href key in release web links")
	}
	releaseURL, ok := webLinkMap["href"].(string)
	if !ok {
		return "", fmt.Errorf("failed to convert release URL to string")
	}
	return releaseURL, nil
}

func extractPublishingInfoFromZip(zipReader *zip.Reader) (string, error) {
	var zf *zip.File
	for _, f := range zipReader.File {
		if strings.HasSuffix(f.Name, publishingInfoJSONFileName) {
			zf = f
			break
		}
	}
	if zf == nil {
		return "", fmt.Errorf("unable to find %s within zip archive", publishingInfoJSONFileName)
	}

	reader, err := zf.Open()
	if err != nil {
		return "", fmt.Errorf("opening zip file %s for reading: %w", zf.Name, err)
	}
	defer reader.Close()

	var b bytes.Buffer
	bufWriter := bufio.NewWriter(&b)
	if _, err = io.Copy(bufWriter, reader); err != nil {
		return "", fmt.Errorf("copying contents of %s to buffer: %w", zf.Name, err)
	}

	info := &PublishingInfo{}
	if err := json.Unmarshal(b.Bytes(), info); err != nil {
		return "", fmt.Errorf("unmarshalling publishing info from %s: %w", zf.Name, err)
	}

	return info.ImageVersion, nil
}
