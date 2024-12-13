package ado

import (
	"fmt"
	"time"
)

type ArtifactBuild struct {
	ID            int
	Name          string
	SourceBranch  string
	SourceVersion string
	FinishTime    time.Time
}

func (b *ArtifactBuild) URL() string {
	if b.ID == 0 {
		return ""
	}
	return fmt.Sprintf("https://msazure.visualstudio.com/%s/_build/results?buildId=%d&view=results", projectName, b.ID)
}

type PublishingInfo struct {
	ImageVersion string `json:"image_version"`
}

type PollingConfig struct {
	BuildCompletionPollInterval *time.Duration
}

func (c *PollingConfig) getBuildCompletionPollInterval() time.Duration {
	if c == nil || c.BuildCompletionPollInterval == nil {
		return 30 * time.Second
	}
	return *c.BuildCompletionPollInterval
}
