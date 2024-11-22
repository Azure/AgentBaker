package gc

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/gc/internal/azure"
	"github.com/Azure/agentbaker/vhdbuilder/gc/internal/env"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

const (
	tagNameNow = "now"
)

var (
	oneDayAgo  = time.Now().Add(-24 * time.Hour)
	oneWeekAgo = time.Now().Add(-7 * 24 * time.Hour)
)

func CollectResourceGroups(ctx context.Context, azureClient azure.Client) error {
	var groupsToCollect []*armresources.ResourceGroup

	groups, err := azureClient.ResourceGroups(ctx)
	if err != nil {
		return fmt.Errorf("getting list of resource groups: %w", err)
	}

	for _, rg := range groups {
		if shouldCollectRGByName(*rg.Name) {
			deletionDeadline := oneDayAgo
			if taggedForSkip(rg.Tags) {
				deletionDeadline = oneWeekAgo
			}

			createdTime, err := getCreatedTime(*rg.Name, rg.Tags)
			if err != nil {
				return err
			}

			if createdTime.Before(deletionDeadline) {
				log.Printf("will attempt to collect resouce group %s", *rg.Name)
				groupsToCollect = append(groupsToCollect, rg)
			}
		}
	}

	for _, rg := range groupsToCollect {
		log.Printf("beginning deletion of resource group %s", *rg.Name)
		if env.Variables.DryRun {
			log.Printf("DRY-RUN: will skip deletion of %s", *rg.Name)
			continue
		}
		azureClient.BeginDeleteResourceGroup(ctx, *rg.Name)
	}

	return nil
}

func unixTimeStringToTime(str string) (time.Time, error) {
	raw, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing unix time string %s as int: %w", str)
	}
	return time.Unix(raw, 0), nil
}

func getCreatedTime(resourceName string, tags map[string]*string) (time.Time, error) {
	if nowValue, ok := tags[tagNameNow]; ok {
		nowTime, err := unixTimeStringToTime(*nowValue)
		if err != nil {
			return time.Time{}, fmt.Errorf("getting unix time from tag value: %w", err)
		}
		return nowTime, nil
	}

	if strings.HasPrefix(resourceName, "vhd-test") || strings.HasPrefix(resourceName, "vhd-scanning") {
		parts := strings.Split(resourceName, "-")
		if len(parts) < 3 {
			return time.Time{}, fmt.Errorf("resource name %s is malformed, cannot determine creation time", resourceName)
		}
		nowTime, err := unixTimeStringToTime(parts[2])
		if err != nil {
			return time.Time{}, fmt.Errorf("getting unix time from resource name: %w", err)
		}
		return nowTime, nil
	}

	return time.Time{}, fmt.Errorf("unable to determine creation time of resource %s", resourceName)
}

func shouldCollectRGByName(rgName string) bool {
	return strings.HasPrefix(rgName, "vhd-test") || strings.HasPrefix(rgName, "vhd-scanning") || strings.HasPrefix(rgName, "pkr-Resource-Group")
}

func taggedForSkip(tags map[string]*string) bool {
	return strings.EqualFold(*tags[env.Variables.SkipTagName], env.Variables.SkipTagValue)
}
