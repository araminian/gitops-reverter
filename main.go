package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func main() {
	commitHash := "f50d95b53a5d9fdb2a1039b6a86aa180ee1afb3d"
	since := time.Now().AddDate(0, -1, 0)
	owner := "trivago"
	repo := "hsw-fork"
	path := "manifests/api/prod"
	basicAuth := &http.BasicAuth{Username: "git", Password: os.Getenv("GITHUB_TOKEN")}

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		log.Fatalf("Failed to create github client: %v", err)
	}
	ignoreBranches := []string{"gitops/sink", "gitops/infra", "gitops/stage", "gitops/seo-indexation", "gitops/member-data"}

	// List all gitops branches
	branches, err := listGitOpsBranches(owner, repo, ignoreBranches)
	if err != nil {
		log.Fatalf("Failed to list gitops branches: %v", err)
	}

	// Get all commits since 1 month ago on master
	commits, err := client.ListCommitsSince(context.Background(), since, "master")
	if err != nil {
		log.Fatalf("Failed to list commits: %v", err)
	}

	masterCommits := processHeadCommits(commits)

	commitGraph, commitsHistory, err := generateCommitGraph(owner, repo, branches, masterCommits, path)
	if err != nil {
		log.Fatalf("Failed to generate commit graph: %v", err)
	}

	for _, commit := range commitGraph {
		fmt.Printf("Commit: %s\n", commit.SHA)
		fmt.Printf("Parent: %s\n", commit.Parent)
		fmt.Printf("Date: %v\n", commit.Date)
		fmt.Printf("GitOps Commits: %v\n", commit.GitOpsCommits)
		fmt.Printf("--------------------------------\n")
	}

	rollbackCommits, err := findRollbackCommits(commitGraph, branches, commitHash)
	if err != nil {
		log.Fatalf("Failed to find rollback commits: %v", err)
	}

	for branch, commit := range rollbackCommits {
		fmt.Printf("Branch: %s\n", branch)
		fmt.Printf("Rollback Commit: %s\n", commit.GitOpsCommit)
		fmt.Printf("Head Commit: %s\n", commit.HeadCommit)
		fmt.Printf("--------------------------------\n")
	}

	log.Printf("Finding commits after rollback")
	commitsAfterRollback, err := findCommitsAfterRollback(rollbackCommits, commitsHistory)
	if err != nil {
		log.Fatalf("Failed to find commits after rollback: %v", err)
	}

	//  Newest to oldest
	for branch, commits := range commitsAfterRollback {
		fmt.Printf("Branch: %s\n", branch)
		fmt.Printf("Number of commits to revert: %d\n", len(commits))

		for i, commit := range commits {
			fmt.Printf("Commit %d: %s\n", i, commit)
		}

		if len(commits) == 0 {
			log.Printf("No commits to revert on branch %s", branch)
			continue
		}

		log.Printf("Reverting %d commits on branch %s", len(commits), branch)
		err = revertFromCommitCLI(owner, repo, basicAuth, branch, commits, true)
		if err != nil {
			log.Fatalf("Failed to revert commit: %v", err)
		}

		fmt.Printf("--------------------------------\n")
	}

}
