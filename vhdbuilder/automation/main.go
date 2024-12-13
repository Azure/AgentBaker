package main

import (
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/automation/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "automation",
	Short: "automation - go binary for fascilitating automated tasks related to daily and official VHD builds + releases",
}

func buildRootCmd() {
	rootCmd.AddCommand(cmd.CreateSIGRelease())
	rootCmd.AddCommand(cmd.CutDaily())
}

func main() {
	buildRootCmd()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
