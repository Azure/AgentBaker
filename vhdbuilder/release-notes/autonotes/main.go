package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

/**
*** This binary autogenerates release notes for AKS VHD releases.
***
*** It accepts:
*** - a run ID from which to download artifacts.
*** - the VHD build date for output naming
*** - a comma-separated list of VHD names to include/ignore.
***
*** Examples:
*** # download ONLY 1804-gen2-gpu release notes from this run ID.
*** autonotes --build 40968951 --include 1804-gen2-gpu
***
*** # download everything EXCEPT 1804-gen2-gpu release notes from this run ID.
*** autonotes --build 40968951 --ignore 1804-gen2-gpu
***
*** # download ONLY 1604,1804,1804-containerd release notes from this run ID.
*** autonotes --build 40968951 --include 1604,1804,1804-containerd
*** # download ONLY 2019-containerd release notes from this run ID.
*** autonotes --build 76289801 --include 2019-containerd
***
*** # download everything EXCEPT 2022-containerd-gen2 release notes from this run ID.
*** autonotes --build 76289801 --ignore 2022-containerd-gen2
***
*** # download ONLY 2022-containerd,2022-containerd-gen2 release notes from this run ID.
*** autonotes --build 76289801 --include 2022-containerd,2022-containerd-gen2
**/

type VhdPublishingInfo struct {
	VhdUrl            string `json:"vhd_url"`
	OsName            string `json:"os_name"`
	SkuName           string `json:"sku_name"`
	OfferName         string `json:"offer_name"`
	HypervGeneration  string `json:"hyperv_generation"`
	ImageArchitecture string `json:"image_architecture"`
	ImageVersion      string `json:"image_version"`
}

func main() {
	var fl flags
	flag.StringVar(&fl.build, "build", "", "run ID for the VHD build.")
	flag.StringVar(&fl.include, "include", "", "only include this list of VHD release notes.")
	flag.StringVar(&fl.ignore, "ignore", "", "ignore release notes for these VHDs")
	flag.StringVar(&fl.path, "path", defaultPath, "output path to root of VHD notes")
	flag.StringVar(&fl.date, "date", defaultDate, "date of VHD build in format YYYYMM.DD.0")
	flag.BoolVar(&fl.skipLatest, "skip-latest", false, "if set, skip creating/updating the latest version of each release artifact for each SKU")

	flag.Parse()

	int := make(chan os.Signal, 1)
	signal.Notify(int, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-int; cancel() }()

	if errs := run(ctx, cancel, &fl); errs != nil {
		for _, err := range errs {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, cancel context.CancelFunc, fl *flags) []error {
	var include, ignore map[string]bool

	includeString := stripWhitespace(fl.include)
	if len(includeString) > 0 {
		include = map[string]bool{}
		includeTokens := strings.Split(includeString, ",")
		for _, token := range includeTokens {
			include[token] = true
		}
	}

	ignoreString := stripWhitespace(fl.ignore)
	if len(ignoreString) > 0 {
		ignore = map[string]bool{}
		ignoreTokens := strings.Split(ignoreString, ",")
		for _, token := range ignoreTokens {
			ignore[token] = true
		}
	}

	enforceInclude := len(include) > 0

	artifactsToDownload := map[string]string{}
	for key, value := range artifactToPath {
		fmt.Printf("%s - %s\n", key, value)
		if ignore[key] {
			continue
		}

		if enforceInclude && !include[key] {
			continue
		}

		artifactsToDownload[key] = value
	}

	var errc = make(chan error)
	var done = make(chan struct{})

	for sku, path := range artifactsToDownload {
		if strings.Contains(path, "AKSWindows") {
			getReleaseNotesWindows(sku, path, fl, errc, done)
		} else {
			getReleaseNotes(sku, path, fl, errc, done)
		}
	}

	var errs []error

	for i := 0; i < len(artifactsToDownload); i++ {
		select {
		case err := <-errc:
			errs = append(errs, err)
		case <-done:
			continue
		}
	}

	return errs
}

func getReleaseNotes(sku, path string, fl *flags, errc chan<- error, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	// working directory, need one per sku because the file name is
	// always "release-notes.txt" so they all overwrite each other.
	tmpdir, err := os.MkdirTemp("", "releasenotes")
	if err != nil {
		errc <- fmt.Errorf("failed to create temp working directory: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	artifactsDirOut := filepath.Join(fl.path, path)

	if err := os.MkdirAll(filepath.Dir(artifactsDirOut), 0644); err != nil {
		errc <- fmt.Errorf("failed to create parent directory %s with error: %s", artifactsDirOut, err)
		return
	}

	if err := os.MkdirAll(artifactsDirOut, 0644); err != nil {
		errc <- fmt.Errorf("failed to create parent directory %s with error: %s", artifactsDirOut, err)
		return
	}

	artifacts := []buildArtifact{
		{
			name:       fmt.Sprintf("vhd-release-notes-%s", sku),
			tempName:   "release-notes.txt",
			outName:    fmt.Sprintf("%s.txt", fl.date),
			latestName: "latest.txt",
		},
		{
			name:       fmt.Sprintf("vhd-image-bom-%s", sku),
			tempName:   "image-bom.json",
			outName:    fmt.Sprintf("%s-image-list.json", fl.date),
			latestName: "latest-image-list.json",
		},
	}

	for _, artifact := range artifacts {
		if err := artifact.process(fl, artifactsDirOut, tmpdir); err != nil {
			fmt.Printf("processing artifact %s for sku %s", artifact.name, sku)
			errc <- fmt.Errorf("failed to process VHD build artifact %s: %w", artifact.name, err)
			return
		}
	}
}

func getReleaseNotesWindows(sku, path string, fl *flags, errc chan<- error, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	releaseNotesName := fmt.Sprintf("vhd-release-notes-%s", sku)
	imageListName := fmt.Sprintf("vhd-image-list-%s", sku)

	artifactsDirOut := filepath.Join(fl.path, path)
	if err := os.MkdirAll(filepath.Dir(artifactsDirOut), 0644); err != nil {
		errc <- fmt.Errorf("failed to create parent directory %s with error: %s", artifactsDirOut, err)
		return
	}

	if err := os.MkdirAll(artifactsDirOut, 0644); err != nil {
		errc <- fmt.Errorf("failed to create parent directory %s with error: %s", artifactsDirOut, err)
		return
	}

	fmt.Printf("downloading releaseNotes '%s' from windows build '%s'\n", releaseNotesName, fl.build)

	cmd := exec.Command("az", "pipelines", "runs", "artifact", "download", "--run-id", fl.build, "--path", artifactsDirOut, "--artifact-name", releaseNotesName)
	if stdout, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Failed downloading releaseNotes '%s' from windows build '%s'\n", releaseNotesName, fl.build)
		errc <- fmt.Errorf("failed to download az devops releaseNotes for sku %s, err: %s, output: %s", sku, err, string(stdout))
		return
	}

	fmt.Printf("downloading imageList '%s' from build '%s'\n", imageListName, fl.build)

	cmd = exec.Command("az", "pipelines", "runs", "artifact", "download", "--run-id", fl.build, "--path", artifactsDirOut, "--artifact-name", imageListName)
	if stdout, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("failed downloading imageList '%s' from windows build '%s'\n", imageListName, fl.build)
		errc <- fmt.Errorf("failed to download az devops imageList for sku %s, err: %s, output: %s", sku, err, string(stdout))
		return
	}
}

func stripWhitespace(str string) string {
	var b strings.Builder
	b.Grow(len(str))
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

type buildArtifact struct {
	// name is the name of the artifact used to download from ADO
	name string
	// tempName is the name of the actual file contained within the artifact bundle we want to extract
	tempName string
	// outName is the versioned name of the artifact file to be uploaded
	outName string
	// latestName is the latest name of the artifact file to be uploaded
	latestName string
}

func (a buildArtifact) process(fl *flags, outdir, tmpdir string) error {
	tempPath := filepath.Join(tmpdir, a.tempName)
	outPath := filepath.Join(outdir, a.outName)

	cmd := exec.Command("az", "pipelines", "runs", "artifact", "download", "--run-id", fl.build, "--path", tmpdir, "--artifact-name", a.name)
	if stdout, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to download az devops releaseNotes %s, err: %s, output: %s", a.name, err, string(stdout))
	}

	if err := os.Rename(tempPath, outPath); err != nil {
		return fmt.Errorf("failed to rename file %s to %s, err: %s", tempPath, outPath, err)
	}

	if !fl.skipLatest {
		data, err := os.ReadFile(outPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s for copying, err: %s", outPath, err)
		}

		latestPath := filepath.Join(outdir, a.latestName)
		if err = os.WriteFile(latestPath, data, 0644); err != nil {
			return fmt.Errorf("failed to copy data from file %s to latest version %s, err: %s", outPath, latestPath, err)
		}
	}

	return nil
}

type flags struct {
	build      string
	include    string // CSV of the map keys below.
	ignore     string // CSV of the map keys below.
	path       string // output path
	date       string // date of vhd build
	skipLatest bool   // whether to skip creating/updating latest version of each artifact for each SKU
}

var defaultPath = filepath.Join("vhdbuilder", "release-notes")
var defaultDate = strings.Split(time.Now().Format("200601.02"), " ")[0] + ".0"

// why does ubuntu use subfolders and mariner doesn't
// there are dependencies on the folder structure but it would
// be nice to fix this.
var artifactToPath = map[string]string{
	"1804-containerd":                      filepath.Join("AKSUbuntu", "gen1", "1804containerd"),
	"1804-gen2-containerd":                 filepath.Join("AKSUbuntu", "gen2", "1804containerd"),
	"1804-gpu-containerd":                  filepath.Join("AKSUbuntu", "gen1", "1804gpucontainerd"),
	"1804-gen2-gpu-containerd":             filepath.Join("AKSUbuntu", "gen2", "1804gpucontainerd"),
	"1804-fips-containerd":                 filepath.Join("AKSUbuntu", "gen1", "1804fipscontainerd"),
	"1804-fips-gen2-containerd":            filepath.Join("AKSUbuntu", "gen2", "1804fipscontainerd"),
	"2004-fips-containerd":                 filepath.Join("AKSUbuntu", "gen1", "2004fipscontainerd"),
	"2004-fips-gen2-containerd":            filepath.Join("AKSUbuntu", "gen2", "2004fipscontainerd"),
	"marinerv1":                            filepath.Join("AKSCBLMariner", "gen1"),
	"marinerv1-gen2":                       filepath.Join("AKSCBLMariner", "gen2"),
	"marinerv2-gen1":                       filepath.Join("AKSCBLMarinerV2", "gen1"),
	"marinerv2-gen1-fips":                  filepath.Join("AKSCBLMarinerV2", "gen1fips"),
	"marinerv2-gen2-fips":                  filepath.Join("AKSCBLMarinerV2", "gen2fips"),
	"marinerv2-gen2":                       filepath.Join("AKSCBLMarinerV2", "gen2"),
	"marinerv2-gen2-kata":                  filepath.Join("AKSCBLMarinerV2", "gen2kata"),
	"marinerv2-gen2-arm64":                 filepath.Join("AKSCBLMarinerV2", "gen2arm64"),
	"marinerv2-gen2-trustedlaunch":         filepath.Join("AKSCBLMarinerV2", "gen2tl"),
	"marinerv2-gen2-kata-trustedlaunch":    filepath.Join("AKSCBLMarinerV2", "gen2katatl"),
	"2004-cvm-gen2-containerd":             filepath.Join("AKSUbuntu", "gen2", "2004cvmcontainerd"),
	"2204-containerd":                      filepath.Join("AKSUbuntu", "gen1", "2204containerd"),
	"2204-gen2-containerd":                 filepath.Join("AKSUbuntu", "gen2", "2204containerd"),
	"2204-arm64-gen2-containerd":           filepath.Join("AKSUbuntu", "gen2", "2204arm64containerd"),
	"2204-tl-gen2-containerd":              filepath.Join("AKSUbuntu", "gen2", "2204tlcontainerd"),
	"2404-containerd":                      filepath.Join("AKSUbuntu", "gen1", "2404containerd"),
	"2404-gen2-containerd":                 filepath.Join("AKSUbuntu", "gen2", "2404containerd"),
	"2404-arm64-gen2-containerd":           filepath.Join("AKSUbuntu", "gen2", "2404arm64containerd"),
	"2404-tl-gen2-containerd":              filepath.Join("AKSUbuntu", "gen2", "2404tlcontainerd"),
	"2019-containerd":                      filepath.Join("AKSWindows", "2019-containerd"),
	"2022-containerd":                      filepath.Join("AKSWindows", "2022-containerd"),
	"2022-containerd-gen2":                 filepath.Join("AKSWindows", "2022-containerd-gen2"),
	"23H2":                                 filepath.Join("AKSWindows", "23H2"),
	"23H2-gen2":                            filepath.Join("AKSWindows", "23H2-gen2"),
	"2025":                                 filepath.Join("AKSWindows", "2025"),
	"2025-gen2":                            filepath.Join("AKSWindows", "2025-gen2"),
	"azurelinuxv2-gen1":                    filepath.Join("AKSAzureLinux", "gen1"),
	"azurelinuxv2-gen2":                    filepath.Join("AKSAzureLinux", "gen2"),
	"azurelinuxv2-gen1-fips":               filepath.Join("AKSAzureLinux", "gen1fips"),
	"azurelinuxv2-gen2-fips":               filepath.Join("AKSAzureLinux", "gen2fips"),
	"azurelinuxv2-gen2-kata":               filepath.Join("AKSAzureLinux", "gen2kata"),
	"azurelinuxv2-gen2-arm64":              filepath.Join("AKSAzureLinux", "gen2arm64"),
	"azurelinuxv2-gen2-trustedlaunch":      filepath.Join("AKSAzureLinux", "gen2tl"),
	"azurelinuxv2-gen2-kata-trustedlaunch": filepath.Join("AKSAzureLinux", "gen2katatl"),
	"azurelinuxv3-gen1":                    filepath.Join("AKSAzureLinuxV3", "gen1"),
	"azurelinuxv3-gen2":                    filepath.Join("AKSAzureLinuxV3", "gen2"),
	"azurelinuxv3-gen1-fips":               filepath.Join("AKSAzureLinuxV3", "gen1fips"),
	"azurelinuxv3-gen2-fips":               filepath.Join("AKSAzureLinuxV3", "gen2fips"),
	"azurelinuxv3-gen2-arm64":              filepath.Join("AKSAzureLinuxV3", "gen2arm64"),
	"azurelinuxv3-gen2-trustedlaunch":      filepath.Join("AKSAzureLinuxV3", "gen2tl"),
}
