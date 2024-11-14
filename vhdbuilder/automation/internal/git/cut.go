package git

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// branch - YYYYMM.DD.PATCH -> daily/vYYYYMMDD
// tag - YYYYMM.DD.PATCH -> v0.YYYYMMDD.daily0
func GetDailyBranchAndTagNames(imageVersion string) (string, string, error) {
	parts := strings.Split(imageVersion, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("cannot get daily branch and tag names from image version %s: malformed", imageVersion)
	}
	date := parts[0] + parts[1]
	return fmt.Sprintf(dailyBranchFormat, date), fmt.Sprintf(dailyTagFormat, date), nil
}

func UpdateImageVersion(repoPath, newVersion string) error {
	fullPath := filepath.Join(repoPath, LinuxSIGImageVersionJSONPath)
	rawVersion, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("reading image version JSON at %s: %w", fullPath, err)
	}

	version := &sigVersion{}
	if err := json.Unmarshal(rawVersion, version); err != nil {
		return fmt.Errorf("unmarshalling %s: %w", fullPath, err)
	}

	version.Version = newVersion
	updatedRaw, err := json.MarshalIndent(version, "", "	")
	if err != nil {
		return fmt.Errorf("marshalling SIG version contents: %w", err)
	}

	if err := os.WriteFile(fullPath, updatedRaw, os.ModePerm); err != nil {
		return fmt.Errorf("writing updated version to %s: %w", fullPath, err)
	}

	return nil
}
