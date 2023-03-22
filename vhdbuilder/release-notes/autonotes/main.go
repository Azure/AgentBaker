package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
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
***
**/

func main() {
	var fl flags
	flag.StringVar(&fl.build, "build", "", "run ID for the VHD build.")
	flag.StringVar(&fl.include, "include", "", "only include this list of VHD release notes.")
	flag.StringVar(&fl.ignore, "ignore", "", "ignore release notes for these VHDs")
	flag.StringVar(&fl.path, "path", defaultPath, "output path to root of VHD notes")
	flag.StringVar(&fl.date, "date", defaultDate, "date of VHD build in format YYYY.MM.DD")

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
		go getReleaseNotes(sku, path, fl, errc, done)
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
	tmpdir, err := ioutil.TempDir("", "releasenotes")
	if err != nil {
		errc <- fmt.Errorf("failed to create temp working directory: %w", err)
	}
	defer os.RemoveAll(tmpdir)

	releaseNotesName := fmt.Sprintf("vhd-release-notes-%s", sku)
	releaseNotesFileIn := filepath.Join(tmpdir, "release-notes.txt")
	imageListName := fmt.Sprintf("vhd-image-bom-%s", sku)
	imageListFileIn := filepath.Join(tmpdir, "image-bom.json")
	artifactsDirOut := filepath.Join(fl.path, path)
	releaseNotesFileOut := filepath.Join(artifactsDirOut, fmt.Sprintf("%s.txt", fl.date))
	imageListFileOut := filepath.Join(artifactsDirOut, fmt.Sprintf("%s-image-list.json", fl.date))
	latestReleaseNotesFile := filepath.Join(artifactsDirOut, "latest.txt")
	latestImageListFile := filepath.Join(artifactsDirOut, "latest-image-list.json")

	if err := os.MkdirAll(artifactsDirOut, 0644); err != nil {
		errc <- fmt.Errorf("failed to create parent directory %s with error: %s", artifactsDirOut, err)
		return
	}

	fmt.Printf("downloading releaseNotes '%s' from build '%s'\n", releaseNotesName, fl.build)

	cmd := exec.Command("az", "pipelines", "runs", "artifact", "download", "--run-id", fl.build, "--path", tmpdir, "--artifact-name", releaseNotesName)
	if stdout, err := cmd.CombinedOutput(); err != nil {
		if err != nil {
			errc <- fmt.Errorf("failed to download az devops releaseNotes for sku %s, err: %s, output: %s", sku, err, string(stdout))
		}
		return
	}

	if err := os.Rename(releaseNotesFileIn, releaseNotesFileOut); err != nil {
		errc <- fmt.Errorf("failed to rename file %s to %s, err: %s", releaseNotesFileIn, releaseNotesFileOut, err)
		return
	}

	data, err := os.ReadFile(releaseNotesFileOut)
	if err != nil {
		errc <- fmt.Errorf("failed to read file %s for copying, err: %s", releaseNotesFileOut, err)
	}

	err = os.WriteFile(latestReleaseNotesFile, data, 0644)
	if err != nil {
		errc <- fmt.Errorf("failed to write file %s for copying, err: %s", releaseNotesFileOut, err)
	}

	cmd = exec.Command("az", "pipelines", "runs", "artifact", "download", "--run-id", fl.build, "--path", tmpdir, "--artifact-name", imageListName)
	if stdout, err := cmd.CombinedOutput(); err != nil {
		if err != nil {
			errc <- fmt.Errorf("failed to download az devops imageList for sku %s, err: %s, output: %s", sku, err, string(stdout))
		}
		return
	}

	if err := os.Rename(imageListFileIn, imageListFileOut); err != nil {
		errc <- fmt.Errorf("failed to rename file %s to %s, err: %s", imageListFileIn, imageListFileOut, err)
		return
	}

	data, err = os.ReadFile(imageListFileOut)
	if err != nil {
		errc <- fmt.Errorf("failed to read file %s for copying, err: %s", releaseNotesFileOut, err)
	}

	err = os.WriteFile(latestImageListFile, data, 0644)
	if err != nil {
		errc <- fmt.Errorf("failed to write file %s for copying, err: %s", releaseNotesFileOut, err)
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

type flags struct {
	build   string
	include string // CSV of the map keys below.
	ignore  string // CSV of the map keys below.
	path    string // output path
	date    string // date of vhd build
}

var defaultPath = filepath.Join("vhdbuilder", "release-notes")
var defaultDate = strings.Split(time.Now().Format("2006.01.02 15:04:05"), " ")[0]

var artifactToPath = map[string]string{
	"1804-containerd":                   filepath.Join("AKSUbuntu", "gen1", "1804containerd"),
	"1804-gen2-containerd":              filepath.Join("AKSUbuntu", "gen2", "1804containerd"),
	"1804-gpu-containerd":               filepath.Join("AKSUbuntu", "gen1", "1804gpucontainerd"),
	"1804-gen2-gpu-containerd":          filepath.Join("AKSUbuntu", "gen2", "1804gpucontainerd"),
	"1804-fips-containerd":              filepath.Join("AKSUbuntu", "gen1", "1804fipscontainerd"),
	"1804-fips-gen2-containerd":         filepath.Join("AKSUbuntu", "gen2", "1804fipscontainerd"),
	"1804-fips-gpu-containerd":          filepath.Join("AKSUbuntu", "gen1", "1804fipsgpucontainerd"),
	"1804-fips-gen2-gpu-containerd":     filepath.Join("AKSUbuntu", "gen2", "1804fipsgpucontainerd"),
	"marinerv1":                         filepath.Join("AKSCBLMariner", "gen1"),
	"marinerv1-gen2":                    filepath.Join("AKSCBLMariner", "gen2"),
	"marinerv2-gen2":                    filepath.Join("AKSCBLMarinerV2", "gen2"),
	"marinerv2-gen2-kata":               filepath.Join("AKSCBLMarinerV2", "gen2kata"),
	"marinerv2-gen2-arm64":              filepath.Join("AKSCBLMarinerV2", "gen2arm64"),
	"marinerv2-gen2-trustedlaunch":      filepath.Join("AKSCBLMarinerV2", "gen2tl"),
	"marinerv2-gen2-kata-trustedlaunch": filepath.Join("AKSCBLMarinerV2", "gen2katatl"),
	"2004-cvm-gen2-containerd":          filepath.Join("AKSUbuntu", "gen2", "2004cvmcontainerd"),
	"2204-containerd":                   filepath.Join("AKSUbuntu", "gen1", "2204containerd"),
	"2204-gen2-containerd":              filepath.Join("AKSUbuntu", "gen2", "2204containerd"),
	"2204-arm64-gen2-containerd":        filepath.Join("AKSUbuntu", "gen2", "2204arm64containerd"),
	"2204-tl-gen2-containerd":           filepath.Join("AKSUbuntu", "gen2", "2204tlcontainerd"),
}
