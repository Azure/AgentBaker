// cosi-extension is a CLI tool for publishing COSI artifacts to PMC Storage
// (via AFD) and registering them in Nebraska for over-the-wire OS updates.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/agentbaker/pkg/nebraska"
	"github.com/spf13/cobra"
)

type publishingInfo struct {
	ArtifactPath      string `json:"artifact_path"`
	SHA256            string `json:"sha256"`
	SizeBytes         int64  `json:"size_bytes"`
	ImageVersion      string `json:"image_version"`
	OSName            string `json:"os_name"`
	SKUName           string `json:"sku_name"`
	OfferName         string `json:"offer_name"`
	ImageArchitecture string `json:"image_architecture"`
	Config            string `json:"config"`
}

var (
	flagNebraskaEndpoint    string
	flagAppID               string
	flagAFDUploadEndpoint   string
	flagAFDDownloadHostname string
	flagContainer           string
	flagPublishingInfo      string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "cosi-extension",
		Short: "Publish COSI artifacts to PMC Storage and register in Nebraska",
	}

	uploadCmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload COSI artifact to PMC Storage through AFD",
		RunE:  runUpload,
	}
	uploadCmd.Flags().StringVar(&flagAFDUploadEndpoint, "afd-upload-endpoint", "", "AFD upload endpoint URL")
	uploadCmd.Flags().StringVar(&flagContainer, "container", "", "Storage container name")
	uploadCmd.Flags().StringVar(&flagPublishingInfo, "publishing-info", "", "Path to cosi-publishing-info.json")
	uploadCmd.MarkFlagRequired("afd-upload-endpoint")
	uploadCmd.MarkFlagRequired("container")
	uploadCmd.MarkFlagRequired("publishing-info")

	registerCmd := &cobra.Command{
		Use:   "register",
		Short: "Register COSI artifact in Nebraska (create package, channel, group)",
		RunE:  runRegister,
	}
	registerCmd.Flags().StringVar(&flagNebraskaEndpoint, "nebraska-endpoint", "", "Nebraska publisher API endpoint URL")
	registerCmd.Flags().StringVar(&flagAppID, "app-id", "", "Nebraska application ID")
	registerCmd.Flags().StringVar(&flagAFDDownloadHostname, "afd-download-hostname", "", "AFD download hostname")
	registerCmd.Flags().StringVar(&flagContainer, "container", "", "Storage container name")
	registerCmd.Flags().StringVar(&flagPublishingInfo, "publishing-info", "", "Path to cosi-publishing-info.json")
	registerCmd.MarkFlagRequired("nebraska-endpoint")
	registerCmd.MarkFlagRequired("app-id")
	registerCmd.MarkFlagRequired("afd-download-hostname")
	registerCmd.MarkFlagRequired("container")
	registerCmd.MarkFlagRequired("publishing-info")

	publishCmd := &cobra.Command{
		Use:   "publish",
		Short: "Upload to storage and register in Nebraska (upload + register)",
		RunE:  runPublish,
	}
	publishCmd.Flags().StringVar(&flagNebraskaEndpoint, "nebraska-endpoint", "", "Nebraska publisher API endpoint URL")
	publishCmd.Flags().StringVar(&flagAppID, "app-id", "", "Nebraska application ID")
	publishCmd.Flags().StringVar(&flagAFDUploadEndpoint, "afd-upload-endpoint", "", "AFD upload endpoint URL")
	publishCmd.Flags().StringVar(&flagAFDDownloadHostname, "afd-download-hostname", "", "AFD download hostname")
	publishCmd.Flags().StringVar(&flagContainer, "container", "", "Storage container name")
	publishCmd.Flags().StringVar(&flagPublishingInfo, "publishing-info", "", "Path to cosi-publishing-info.json")
	publishCmd.MarkFlagRequired("nebraska-endpoint")
	publishCmd.MarkFlagRequired("app-id")
	publishCmd.MarkFlagRequired("afd-upload-endpoint")
	publishCmd.MarkFlagRequired("afd-download-hostname")
	publishCmd.MarkFlagRequired("container")
	publishCmd.MarkFlagRequired("publishing-info")

	rootCmd.AddCommand(uploadCmd, registerCmd, publishCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadPublishingInfo(path string) (*publishingInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read publishing info %s: %w", path, err)
	}
	var info publishingInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parse publishing info: %w", err)
	}
	return &info, nil
}

func blobName(info *publishingInfo) string {
	return fmt.Sprintf("%s-%s.cosi", info.Config, info.ImageVersion)
}

func archCode(arch string) int {
	switch strings.ToUpper(arch) {
	case "ARM64":
		return nebraska.ArchARM64
	default:
		return nebraska.ArchAMD64
	}
}

func newNebraskaTokenProvider() nebraska.TokenProvider {
	return func(_ context.Context) (string, error) {
		return os.Getenv("NEBRASKA_TOKEN"), nil
	}
}

func runUpload(cmd *cobra.Command, _ []string) error {
	info, err := loadPublishingInfo(flagPublishingInfo)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	sc := nebraska.NewStorageClient(flagAFDUploadEndpoint, nil)

	blob := blobName(info)
	fmt.Printf("Uploading %s to %s/%s/%s\n", info.ArtifactPath, flagAFDUploadEndpoint, flagContainer, blob)
	if err := sc.UploadCOSIArtifact(ctx, flagContainer, blob, info.ArtifactPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	fmt.Println("Upload successful")
	return nil
}

func runRegister(cmd *cobra.Command, _ []string) error {
	info, err := loadPublishingInfo(flagPublishingInfo)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	client := nebraska.NewClient(flagNebraskaEndpoint, newNebraskaTokenProvider(), nil)

	blob := blobName(info)
	downloadURL := nebraska.DeriveDownloadURL(flagAFDDownloadHostname, flagContainer, blob)
	arch := archCode(info.ImageArchitecture)

	// 1. Create package
	pkg := nebraska.Package{
		Type:              nebraska.PackageTypeFlatcar,
		Version:           info.ImageVersion,
		URL:               downloadURL,
		Filename:          blob,
		Size:              fmt.Sprintf("%d", info.SizeBytes),
		Hash:              info.SHA256,
		Arch:              arch,
		ApplicationID:     flagAppID,
		ChannelsBlacklist: []string{},
		FlatcarAction: &nebraska.FlatcarAction{
			Event:  "postinstall",
			Sha256: info.SHA256,
		},
	}
	fmt.Printf("Creating package %s (version %s)...\n", blob, info.ImageVersion)
	createdPkg, err := client.CreatePackage(ctx, flagAppID, pkg)
	if err != nil {
		return fmt.Errorf("create package: %w", err)
	}
	fmt.Printf("Package created: %s\n", createdPkg.ID)

	// 2. Create pinned channel
	pinnedChName := fmt.Sprintf("pin-%s", info.ImageVersion)
	fmt.Printf("Creating pinned channel %s...\n", pinnedChName)
	pinnedCh, err := client.CreateChannel(ctx, flagAppID, nebraska.Channel{
		Name:          pinnedChName,
		Color:         "#1f78b4",
		Arch:          arch,
		ApplicationID: flagAppID,
		PackageID:     createdPkg.ID,
	})
	if err != nil {
		return fmt.Errorf("create pinned channel: %w", err)
	}
	fmt.Printf("Pinned channel created: %s\n", pinnedCh.ID)

	// 3. Create pinned group
	pinnedGrpName := fmt.Sprintf("pin-%s", info.ImageVersion)
	fmt.Printf("Creating pinned group %s...\n", pinnedGrpName)
	_, err = client.CreateGroup(ctx, flagAppID, nebraska.Group{
		Name:                      pinnedGrpName,
		Description:               fmt.Sprintf("Pinned to version %s", info.ImageVersion),
		ApplicationID:             flagAppID,
		ChannelID:                 pinnedCh.ID,
		PolicyUpdatesEnabled:      true,
		PolicySafeMode:            true,
		PolicyOfficeHours:         false,
		PolicyTimezone:            "UTC",
		PolicyPeriodInterval:      "1 hours",
		PolicyMaxUpdatesPerPeriod: 100,
		PolicyUpdateTimeout:       "1 hours",
	})
	if err != nil {
		return fmt.Errorf("create pinned group: %w", err)
	}
	fmt.Println("Pinned group created")

	// 4. Update "latest" channel
	channels, err := client.ListChannels(ctx, flagAppID)
	if err != nil {
		return fmt.Errorf("list channels: %w", err)
	}
	for _, ch := range channels {
		if ch.Name == "latest" {
			fmt.Printf("Updating 'latest' channel to point to package %s...\n", createdPkg.ID)
			ch.PackageID = createdPkg.ID
			if _, err := client.UpdateChannel(ctx, flagAppID, ch.ID, ch); err != nil {
				return fmt.Errorf("update latest channel: %w", err)
			}
			fmt.Println("Latest channel updated")
			break
		}
	}

	fmt.Printf("Registration complete: version %s registered in Nebraska\n", info.ImageVersion)
	return nil
}

func runPublish(cmd *cobra.Command, args []string) error {
	if err := runUpload(cmd, args); err != nil {
		return err
	}
	return runRegister(cmd, args)
}
