package suite

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Azure/agentbakere2e/toolkit"
)

const (
	publishingInfoDirName = "publishinginfo"

	buildIDEnvironmentVarName            = "BUILD_ID"
	adoPATEnvironmentVarName             = "ADO_PAT"
	subscriptionIdEnvironmentVarName     = "SUBSCRIPTION_ID"
	locationEnvironmentVarName           = "LOCATION"
	vhdBuildIDEnvironmentVarName         = "VHD_BUILD_ID"
	keepVMSSEnvironmentVarName           = "KEEP_VMSS"
	scenariosToRunEnvironmentVarName     = "SCENARIOS_TO_RUN"
	scenariosToExcludeEnvironmentVarName = "SCENARIOS_TO_EXCLUDE"

	suiteConfigStringTemplate = `subscription: %[1]s,
location: %[2]s,
resource group: %[3]s,
keep vmss: %[4]t`
)

type Config struct {
	BuildID            string
	Subscription       string
	Location           string
	ResourceGroupName  string
	PAT                string
	PublishingInfoDir  string
	VHDBuildID         int
	ScenariosToRun     map[string]bool
	ScenariosToExclude map[string]bool
	KeepVMSS           bool
}

func (c *Config) String() string {
	return fmt.Sprintf(suiteConfigStringTemplate, c.Subscription, c.Location, c.ResourceGroupName, c.KeepVMSS)
}

func (c *Config) UseVHDsFromBuild() bool {
	return c.VHDBuildID != 0
}

func NewConfigForEnvironment() (*Config, error) {
	config := &Config{}

	sub, err := mustGetEnv(subscriptionIdEnvironmentVarName)
	if err != nil {
		return nil, err
	}

	location, err := mustGetEnv(locationEnvironmentVarName)
	if err != nil {
		return nil, err
	}

	config.Subscription = sub
	config.Location = location
	config.BuildID = os.Getenv(buildIDEnvironmentVarName)
	config.ResourceGroupName = fmt.Sprintf(abe2eResourceGroupNameTemplate, location)
	config.KeepVMSS = strings.EqualFold(os.Getenv(keepVMSSEnvironmentVarName), "true")

	if vhdBuildID := os.Getenv(vhdBuildIDEnvironmentVarName); vhdBuildID != "" {
		pat := os.Getenv(adoPATEnvironmentVarName)
		if pat == "" {
			return nil, fmt.Errorf("ADO_PAT must be specified in environment when running E2E suite from custom VHD build (%s currently set to %s)",
				vhdBuildIDEnvironmentVarName, vhdBuildID)
		}

		bid, err := strconv.Atoi(vhdBuildID)
		if err != nil {
			return nil, fmt.Errorf("unable to convert %s env var from string to integer: %w", vhdBuildIDEnvironmentVarName, err)
		}

		config.PAT = pat
		config.VHDBuildID = bid
	}

	include := os.Getenv(scenariosToRunEnvironmentVarName)
	exclude := os.Getenv(scenariosToExcludeEnvironmentVarName)

	// enforce SCENARIOS_TO_RUN over SCENARIOS_TO_EXCLUDE
	if include != "" {
		config.ScenariosToRun = toolkit.StrToBoolMap(include)
	} else if exclude != "" {
		config.ScenariosToExclude = toolkit.StrToBoolMap(exclude)
	}

	return config, nil
}

func mustGetEnv(varName string) (string, error) {
	val := os.Getenv(varName)
	if val == "" {
		return "", fmt.Errorf("missing required environment variable %q", varName)
	}
	return val, nil
}
