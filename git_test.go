package main

import (
	"os"
	"testing"
	"time"
)

func TestCloneRepositoryCLI(t *testing.T) {

	owner := "trivago"
	repoName := "hotel-search-web"
	t.Logf("Cloning repository %s/%s", owner, repoName)
	start := time.Now()
	repoDir, err := cloneRepositoryCLI(owner, repoName)
	defer os.RemoveAll(repoDir)
	if err != nil {
		t.Fatalf("Failed to clone repository: %v", err)
	}
	t.Logf("Repository cloned in %v in directory %s", time.Since(start), repoDir)
}

func TestCreateWorktree(t *testing.T) {

	owner := "trivago"
	repoName := "hotel-search-web"

	branch := "gitops/advertisers"
	branchDir, err := os.MkdirTemp("", "git-revert-*")
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}

	t.Logf("Creating temporary directory %s", branchDir)
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Fatalf("GITHUB_TOKEN environment variable is not set")
	}

	start := time.Now()
	repoDir, err := cloneRepositoryCLI(owner, repoName)
	if err != nil {
		t.Fatalf("Failed to clone repository: %v", err)
	}

	t.Logf("Repository cloned in %v in directory %s", time.Since(start), repoDir)

	t.Logf("Creating worktree for branch %s", branch)

	err = createWorktree(repoDir, branch, branchDir)
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}
	t.Logf("Worktree created in %v", time.Since(start))
}
