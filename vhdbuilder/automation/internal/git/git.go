package git

import (
	"fmt"
	"os"
	"time"

	"github.com/Azure/agentbaker/vhdbuilder/automation/internal/env"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type Client struct {
	pat string
}

func NewClient() (*Client, error) {
	if env.Vars.GitHubPAT == "" {
		return nil, fmt.Errorf("cannot construct git client: GITHUB_PAT missing from environment")
	}
	return &Client{
		pat: env.Vars.GitHubPAT,
	}, nil
}

func (c *Client) Clone(repoName, repoURL string) (*git.Repository, string, error) {
	cloneDir, err := os.MkdirTemp("", "git")
	if err != nil {
		return nil, "", fmt.Errorf("creating temp dir: %w", err)
	}

	repo, err := git.PlainClone(cloneDir, false, &git.CloneOptions{
		Auth:     c.auth(),
		URL:      repoURL,
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, "", fmt.Errorf("cloning repo from %s: %w", repoURL, err)
	}

	return repo, cloneDir, nil
}

func (c *Client) CheckoutNewBranchFromCommit(repo *git.Repository, commitHash, newBranchName string, overwriteExisting bool) error {
	exists, err := c.remoteRefExists(repo, newBranchName, false)
	if err != nil {
		return fmt.Errorf("checking if branch %s exists on remote: %w", newBranchName, err)
	}
	if exists && !overwriteExisting {
		return fmt.Errorf("branch %s already exists on the remote, --overwrite-existing-branch must be specified", newBranchName)
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting repo worktree: %w", err)
	}

	if exists {
		checkoutOpts := &git.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(newBranchName),
			Force:  true,
		}
		if err := w.Checkout(checkoutOpts); err != nil {
			remote, err := repo.Remote("origin")
			if err != nil {
				return fmt.Errorf("getting remote: %w", err)
			}

			refSpecStr := fmt.Sprintf("refs/heads/%s:refs/heads/%s", newBranchName, newBranchName)
			refSpecs := []config.RefSpec{config.RefSpec(refSpecStr)}

			if err := remote.Fetch(&git.FetchOptions{
				RefSpecs: refSpecs,
			}); err != nil {
				if err != git.NoErrAlreadyUpToDate {
					return fmt.Errorf("fetching origin from remote: %w", err)
				}
			}

			if err := w.Checkout(checkoutOpts); err != nil {
				return fmt.Errorf("checking out branch %s: %w", newBranchName, err)
			}
		}
	} else {
		err = w.Checkout(&git.CheckoutOptions{
			Hash:   plumbing.NewHash(commitHash),
			Branch: plumbing.NewBranchReferenceName(newBranchName),
			Create: true,
		})
		if err != nil {
			return fmt.Errorf("checking out new branch %s from hash %s: %w", newBranchName, commitHash, err)
		}
	}

	return nil
}

func (c *Client) AddAllAndCommit(repo *git.Repository, commitMessage string) error {
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("getting repo worktree: %w", err)
	}

	if _, err := w.Add("."); err != nil {
		return fmt.Errorf("adding changes from %q: %w", ".", err)
	}

	_, err = w.Commit(commitMessage, &git.CommitOptions{
		Author: c.signature(),
	})
	if err != nil {
		return fmt.Errorf("committing local changes: %w", err)
	}

	return nil
}

func (c *Client) PushNewBranch(repo *git.Repository, branchName string) error {
	exists, err := c.remoteRefExists(repo, branchName, false)
	if err != nil {
		return fmt.Errorf("checking if branch %s exists on remote: %w", branchName, err)
	}

	branchRef := plumbing.NewBranchReferenceName(branchName)
	opts := &git.PushOptions{
		Auth:     c.auth(),
		Progress: os.Stdout,
	}
	if !exists {
		opts.RemoteName = "origin"
		opts.RefSpecs = []config.RefSpec{
			config.RefSpec(branchRef + ":" + branchRef),
		}
	}

	if err := repo.Push(opts); err != nil {
		return fmt.Errorf("pushing branch to origin: %w", err)
	}

	return nil
}

func (c *Client) CreateAndPushTag(repo *git.Repository, tagName, tagMessage string) error {
	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("getting current repo HEAD: %w", err)
	}

	_, err = repo.CreateTag(tagName, head.Hash(), &git.CreateTagOptions{
		Message: tagMessage,
		Tagger:  c.signature(),
	})
	if err != nil {
		return fmt.Errorf("creating local tag %s: %w", tagName, err)
	}

	err = repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", tagName, tagName)),
		},
		Auth:     c.auth(),
		Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("pushing tag %s to remote: %w", tagName, err)
	}

	return nil
}

func (c *Client) TagExists(repo *git.Repository, tagName string) (bool, error) {
	return c.remoteRefExists(repo, tagName, true)
}

func (c *Client) remoteRefExists(repo *git.Repository, refName string, isTag bool) (bool, error) {
	remote, err := repo.Remote("origin")
	if err != nil {
		return false, fmt.Errorf("getting remote origin: %w", err)
	}

	refs, err := remote.List(&git.ListOptions{
		Auth: c.auth(),
	})
	if err != nil {
		return false, fmt.Errorf("listing refs from remote: %w", err)
	}

	for _, ref := range refs {
		if isTag {
			if ref.Name().IsTag() && ref.Name().Short() == refName {
				return true, nil
			}
		} else {
			if ref.Name().IsBranch() && ref.Name().Short() == refName {
				return true, nil
			}
		}
	}

	return false, nil
}

func (c *Client) auth() *http.BasicAuth {
	return &http.BasicAuth{
		Username: "aks-node",
		Password: c.pat,
	}
}

func (c *Client) signature() *object.Signature {
	return &object.Signature{
		Name:  "aks-node",
		Email: "aks-node@microsoft.com",
		When:  time.Now(),
	}
}
