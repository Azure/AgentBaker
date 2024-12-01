package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/ado"
	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/git"
	"github.com/spf13/cobra"
)

const (
	updateImageVersionCommitMessage = "chore: update image version on daily branch"
)

const (
	dailyTagMessageFormat = "daily tag for image version %s"
)

func cutDaily(flags cutDailyCommandFlags) error {
	ctx := context.Background()

	adoClient, err := ado.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("constructing ADO client: %w", err)
	}

	gitClient, err := git.NewClient()
	if err != nil {
		return fmt.Errorf("constructing git client: %w", err)
	}

	log.Println("getting target daily build...")
	vhdBuild, buildURL, err := adoClient.GetLatestVHDBuildFromBranch(ctx, flags.targetBranch)
	if err != nil {
		return fmt.Errorf("getting latest successful VHD build from branch %s: %w", flags.targetBranch, err)
	}

	vhdBuildID := *vhdBuild.Id
	vhdBuildCommitHash := *vhdBuild.SourceVersion
	log.Printf("chose daily build %d with commit hash %s: %s", vhdBuildID, vhdBuildCommitHash, buildURL)

	log.Printf("getting image version for VHD build %d...", vhdBuildID)
	imageVersion, err := adoClient.GetImageVersionForVHDBuild(ctx, vhdBuildID)
	if err != nil {
		return fmt.Errorf("getting image version for VHD build %d: %w", vhdBuildID, err)
	}
	log.Printf("image version for VHD build %d is %s", vhdBuildID, imageVersion)

	dailyBranchName, dailyTag, err := git.GetDailyBranchAndTagNames(imageVersion)
	if err != nil {
		return fmt.Errorf("getting daily branch name from image version: %w", err)
	}

	log.Printf("cloning %s...", git.AgentBakerRepoName)
	repo, repoPath, err := gitClient.Clone(git.AgentBakerRepoName, git.AgentBakerRepoURL)
	if err != nil {
		return fmt.Errorf("cloning AgentBaker repo to disk: %w", err)
	}
	if !flags.preserveAgentBakerClone {
		defer func() {
			log.Printf("cleaning up local AgentBaker clone at %s...", repoPath)
			if err := os.RemoveAll(repoPath); err != nil {
				log.Fatal(err)
			}
		}()
	}
	log.Printf("cloned %s into %s", git.AgentBakerRepoName, repoPath)

	tagExists, err := gitClient.TagExists(repo, dailyTag)
	if err != nil {
		return fmt.Errorf("checking if tag %s exists on remote: %w", dailyTag, err)
	}
	if tagExists {
		log.Printf("daily tag %s already exists on remote, nothing to do", dailyTag)
		return nil
	}

	log.Printf("checking out new branch %s...", dailyBranchName)
	if err := gitClient.CheckoutNewBranchFromCommit(repo, vhdBuildCommitHash, dailyBranchName, flags.overwriteExistingBranch); err != nil {
		return fmt.Errorf("checking out new branch from commit %s: %w", vhdBuildCommitHash, err)
	}

	if err := git.UpdateImageVersion(repoPath, imageVersion); err != nil {
		return fmt.Errorf("updating image version on local branch %s: %w", dailyBranchName, err)
	}

	log.Printf("committing local changes...")
	if err := gitClient.AddAllAndCommit(repo, updateImageVersionCommitMessage); err != nil {
		return fmt.Errorf("committing local changes on branch %s: %w", dailyBranchName, err)
	}

	log.Printf("pushing new branch %s to origin...", dailyBranchName)
	if err := gitClient.PushNewBranch(repo, dailyBranchName); err != nil {
		return fmt.Errorf("pushing new branch %s to origin: %w", dailyBranchName, err)
	}

	log.Printf("creating and pushing new tag %s to origin", dailyTag)
	tagMsg := fmt.Sprintf(dailyTagMessageFormat, imageVersion)
	if err := gitClient.CreateAndPushTag(repo, dailyTag, tagMsg); err != nil {
		return fmt.Errorf("creating and pushing tag %s to origin: %w", dailyTag, err)
	}

	return nil
}

type cutDailyCommandFlags struct {
	targetBranch            string
	overwriteExistingBranch bool
	preserveAgentBakerClone bool
}

func (o cutDailyCommandFlags) validate() error {
	if o.targetBranch == "" {
		return fmt.Errorf("target branch must be specified to perform a daily cut")
	}
	return nil
}

func CutDaily() *cobra.Command {
	var (
		flags = cutDailyCommandFlags{}
	)

	cmd := &cobra.Command{
		Use:   "cut-daily",
		Short: "cut a daily branch and tag for the most-recently completed VHD build off a particular branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := flags.validate(); err != nil {
				return err
			}
			return cutDaily(flags)
		},
	}

	cmd.Flags().StringVar(&flags.targetBranch, "target-branch", "master", "target branch from which the daily build will be taken")
	cmd.Flags().BoolVar(&flags.preserveAgentBakerClone, "preserve-agentbaker-clone", false, "preserve the local clone of AgentBaker on disk")
	cmd.Flags().BoolVar(&flags.overwriteExistingBranch, "overwrite-existing-branch", false, "overwrite the contents of the target branch if it already exists")
	return cmd
}
