package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

func cloneRepoBranch(url string, branch string, depth int, auth *http.BasicAuth) (*git.Repository, error) {
	refName := plumbing.NewBranchReferenceName(branch)
	repo, err := git.Clone(
		memory.NewStorage(),
		nil,
		&git.CloneOptions{
			URL:           url,
			ReferenceName: refName,
			SingleBranch:  true,
			Depth:         depth,
			Tags:          git.NoTags,
			Auth:          auth,
		},
	)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

// revertFromCommitCLI reverts multiple commits in a single command
func revertFromCommitCLI(owner, repoName string, auth *http.BasicAuth, branch string, commits []string, force bool, pushMode bool) error {
	// Check if commits slice is empty
	if len(commits) == 0 {
		return fmt.Errorf("no commits provided to revert")
	}

	url := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)

	// Create a temporary directory to clone the repository
	tempDir, err := os.MkdirTemp("", "git-revert-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up when we're done

	log.Printf("Cloning repository %s", url)
	// Clone the repository using go-git library
	refName := plumbing.NewBranchReferenceName(branch)
	repo, err := git.PlainClone(tempDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: refName,
		SingleBranch:  true,
		Auth:          auth,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}
	log.Printf("Repository cloned successfully in %s", tempDir)

	// Add author information for git commands
	// Execute git revert command using CLI for all commits at once
	revertArgs := []string{"revert", "--no-edit"}

	if force {
		revertArgs = append(revertArgs, "--no-gpg-sign")
	}

	// Add all commits to revert in a single command (from newest to oldest)
	revertArgs = append(revertArgs, commits...)

	revertCmd := exec.Command("git", revertArgs...)
	log.Printf("Revert Command: %v", revertCmd.String())
	revertCmd.Dir = tempDir
	revertOutput, err := revertCmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to revert commits: %s, %w", revertOutput, err)
	}

	// Get the worktree
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Make sure all new files are added
	_, err = wt.Add(".")
	if err != nil {
		return fmt.Errorf("failed to add files to index: %w", err)
	}

	// Push the changes back using go-git
	if !pushMode {
		log.Printf("Skipping push of changes to remote repository, pushMode is false")
		return nil
	}
	err = repo.Push(&git.PushOptions{
		Auth:  auth,
		Force: force,
	})
	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	return nil
}
