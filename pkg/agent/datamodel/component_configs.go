package datamodel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Components struct {
	ContainerImages []*ContainerImage `json:"ContainerImages"`
	DownloadFiles   []DownloadFile    `json:"DownloadFiles"`
}

type ContainerImage struct {
	DownloadURL       string   `json:"downloadURL"`
	Amd64OnlyVersions []string `json:"amd64OnlyVersions"`
	MultiArchVersions []string `json:"multiArchVersions"`
}

type DownloadFile struct {
	FileName               string   `json:"fileName"`
	DownloadLocation       string   `json:"downloadLocation"`
	DownloadURL            string   `json:"downloadURL"`
	Versions               []string `json:"versions"`
	TargetContainerRuntime string   `json:"targetContainerRuntime,omitempty"`
}

type DockerKubeProxyImages struct {
	ContainerImages []*ContainerImage `json:"ContainerImages"`
}

type KubeProxyImages struct {
	DockerKubeProxyImages     *DockerKubeProxyImages `json:"dockerKubeProxyImages"`
	ContainerdKubeProxyImages *DockerKubeProxyImages `json:"containerdKubeProxyImages"`
}

func loadJsonFromFile(path string, v interface{}) error {
	configFile, err := os.Open(path)
	defer configFile.Close()

	if err != nil {
		return err
	}

	jsonParser := json.NewDecoder(configFile)
	return jsonParser.Decode(&v)
}

func toImageList(downloadURL string, imageTagList []string) ([]string, error) {
	ret := []string{}

	if !strings.Contains(downloadURL, "*") {
		return ret, fmt.Errorf("downloadURL does not contain *")
	}

	for _, tag := range imageTagList {
		ret = append(ret, strings.Replace(downloadURL, "*", tag, 1))
	}

	return ret, nil
}

// begins Components

func NewComponentsFromFile(path string) (*Components, error) {
	ret := &Components{}

	err := loadJsonFromFile(path, ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (c *Components) ToImageList() []string {
	ret := []string{}

	if c.ContainerImages != nil {
		for _, image := range c.ContainerImages {
			if image.Amd64OnlyVersions != nil {
				amd64OnlyImageList, _ := toImageList(image.DownloadURL, image.Amd64OnlyVersions)
				ret = append(ret, amd64OnlyImageList...)
			}

			if image.MultiArchVersions != nil {
				multiArchImageList, _ := toImageList(image.DownloadURL, image.MultiArchVersions)
				ret = append(ret, multiArchImageList...)
			}
		}
	}
	return ret
}

// ends Components

// begins KubeProxyImages

func NewKubeProxyImagesFromFile(path string) (*KubeProxyImages, error) {
	ret := &KubeProxyImages{}

	err := loadJsonFromFile(path, ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (k *KubeProxyImages) ToImageList() []string {
	ret := []string{}

	if k.DockerKubeProxyImages != nil && k.DockerKubeProxyImages.ContainerImages != nil {
		for _, image := range k.DockerKubeProxyImages.ContainerImages {
			if image.Amd64OnlyVersions != nil {
				amd64OnlyImageList, _ := toImageList(image.DownloadURL, image.Amd64OnlyVersions)
				ret = append(ret, amd64OnlyImageList...)
			}

			if image.MultiArchVersions != nil {
				multiArchImageList, _ := toImageList(image.DownloadURL, image.MultiArchVersions)
				ret = append(ret, multiArchImageList...)
			}
		}
	}

	if k.ContainerdKubeProxyImages != nil && k.ContainerdKubeProxyImages.ContainerImages != nil {
		for _, image := range k.ContainerdKubeProxyImages.ContainerImages {
			if image.Amd64OnlyVersions != nil {
				amd64OnlyImageList, _ := toImageList(image.DownloadURL, image.Amd64OnlyVersions)
				ret = append(ret, amd64OnlyImageList...)
			}

			if image.MultiArchVersions != nil {
				multiArchImageList, _ := toImageList(image.DownloadURL, image.MultiArchVersions)
				ret = append(ret, multiArchImageList...)
			}
		}
	}

	return ret
}
