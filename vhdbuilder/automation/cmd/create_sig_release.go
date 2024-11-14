package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/ado"
	"github.com/spf13/cobra"
)

func createSIGRelease(opts createOfficialSIGReleaseFlags) error {
	ctx := context.Background()

	adoClient, err := ado.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("constructing ADO client: %w", err)
	}

	log.Printf("building EV2 artifacts for VHD build %s...", opts.vhdBuildID)
	build, err := adoClient.BuildEV2Artifacts(ctx, opts.vhdBuildID, nil)
	if err != nil {
		return fmt.Errorf("building EV2 artifacts: %w", err)
	}

	log.Printf("creating SIG release for artifact build %s...", build.Name)
	if err := adoClient.CreateSIGRelease(ctx, build); err != nil {
		return fmt.Errorf("creating SIG release: %w", err)
	}

	return nil
}

type createOfficialSIGReleaseFlags struct {
	vhdBuildID string
}

func (o createOfficialSIGReleaseFlags) validate() error {
	if o.vhdBuildID == "" {
		return fmt.Errorf("VHD build ID must be specified to create an official SIG release")
	}
	return nil
}

func CreateSIGRelease() *cobra.Command {
	var (
		flags = createOfficialSIGReleaseFlags{}
	)

	cmd := &cobra.Command{
		Use:   "create-sig-release",
		Short: "create an official SIG release",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			return createSIGRelease(flags)
		},
	}

	cmd.Flags().StringVar(&flags.vhdBuildID, "vhd-build-id", "", "ID of the VHD build used to create the release")
	return cmd
}
