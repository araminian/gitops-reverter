package main

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestListCommitsSince(t *testing.T) {

	owner := "trivago"
	repo := "hotel-search-web"
	branch := "master"
	client, err := NewGithubClient(owner, repo)
	if err != nil {
		t.Fatalf("Failed to create github client: %v", err)
	}

	// 17 Apr 2025 , 5PM , CET = GMT + 2
	since := time.Date(2025, 4, 17, 17, 0, 0, 0, time.FixedZone("CET", 2*3600))

	commits, err := client.ListCommitsSince(context.Background(), since, branch)
	if err != nil {
		t.Fatalf("Failed to list commits: %v", err)
	}

	for _, commit := range commits {
		fmt.Println(commit.GetSHA())
		fmt.Printf("Local Time: %v\n", commit.GetCommit().GetAuthor().GetDate().In(time.FixedZone("CET", 2*3600)))
	}
}

func TestListAllWorkflowsRuns(t *testing.T) {

	owner := "trivago"
	repo := "hotel-search-web"

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		t.Fatalf("Failed to create github client: %v", err)
	}

	workflowsRuns, err := client.ListAllWorkflowsRuns(context.Background(), "master")
	if err != nil {
		t.Fatalf("Failed to list workflows runs: %v", err)
	}

	for _, workflowRun := range workflowsRuns {
		fmt.Println(workflowRun.GetID())
		fmt.Println(workflowRun.GetName())
		fmt.Println(workflowRun.GetStatus())
		fmt.Println(workflowRun.GetURL())
		fmt.Println(workflowRun.GetCancelURL())
		fmt.Println(workflowRun.GetWorkflowURL())
		fmt.Println(workflowRun.GetWorkflowID())
	}
}

func TestMasterHistory(t *testing.T) {
	owner := "trivago"
	repo := "hotel-search-web"
	branch := "master"

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		t.Fatalf("Failed to create github client: %v", err)
	}

	since := time.Now().AddDate(0, -1, 0)

	commits, err := client.ListCommitsSince(context.Background(), since, branch)
	if err != nil {
		t.Fatalf("Failed to list commits: %v", err)
	}

	masterCommits := processHeadCommits(commits)

	for _, commit := range masterCommits {
		fmt.Printf("Commit: %s\n", commit.SHA)
		fmt.Printf("Parent: %s\n", commit.Parent)
		fmt.Printf("Date: %v\n", commit.Date)
		fmt.Printf("--------------------------------\n")
	}
}

func TestListGitOpsBranches(t *testing.T) {
	owner := "trivago"
	repo := "hotel-search-web"
	ignore := []string{"gitops/sink", "gitops/infra", "gitops/stage"}
	branches, err := listGitOpsBranches(owner, repo, ignore)
	if err != nil {
		t.Fatalf("Failed to list gitops branches: %v", err)
	}

	for _, branch := range branches {
		fmt.Printf("GitOps Branch: %s\n", branch)
	}

}

func TestGenerateCommitGraph(t *testing.T) {

	owner := "trivago"
	repo := "hotel-search-web"
	client, err := NewGithubClient(owner, repo)
	if err != nil {
		t.Fatalf("Failed to create github client: %v", err)
	}
	ignore := []string{"gitops/sink", "gitops/infra", "gitops/stage", "gitops/seo-indexation"}

	// List all gitops branches
	branches, err := listGitOpsBranches(owner, repo, ignore)
	if err != nil {
		t.Fatalf("Failed to list gitops branches: %v", err)
	}

	// Get all commits since 1 month ago on master
	since := time.Now().AddDate(0, -1, 0)
	commits, err := client.ListCommitsSince(context.Background(), since, "master")
	if err != nil {
		t.Fatalf("Failed to list commits: %v", err)
	}

	masterCommits := processHeadCommits(commits)

	path := "manifests/api/prod"
	commitGraph, commitsHistory, err := generateCommitGraph(owner, repo, branches, masterCommits, path)
	if err != nil {
		t.Fatalf("Failed to generate commit graph: %v", err)
	}

	_ = commitsHistory

	// for _, commit := range commitGraph {
	// 	fmt.Printf("Commit: %s\n", commit.SHA)
	// 	fmt.Printf("Parent: %s\n", commit.Parent)
	// 	fmt.Printf("Date: %v\n", commit.Date)
	// 	fmt.Printf("GitOps Commits: %v\n", commit.GitOpsCommits)
	// 	fmt.Printf("--------------------------------\n")
	// }

	t.Logf("Finding rollback commits")
	candidateCommit := "aee08b8b8a6c8212a043486c78af8fe7528464af"

	rollbackCommits, err := findRollbackCommits(commitGraph, branches, candidateCommit)

	if err != nil {
		t.Fatalf("Failed to find rollback commits: %v", err)
	}

	for branch, commit := range rollbackCommits {
		fmt.Printf("Branch: %s\n", branch)
		fmt.Printf("Rollback Commit: %s\n", commit.GitOpsCommit)
		fmt.Printf("Head Commit: %s\n", commit.HeadCommit)
		fmt.Printf("--------------------------------\n")
	}

	t.Logf("Finding commits after rollback")
	commitsAfterRollback, err := findCommitsAfterRollback(rollbackCommits, commitsHistory)
	if err != nil {
		t.Fatalf("Failed to find commits after rollback: %v", err)
	}

	//  Newest to oldest
	for branch, commits := range commitsAfterRollback {
		fmt.Printf("Branch: %s\n", branch)
		for i, commit := range commits {
			fmt.Printf("Commit %d: %s\n", i, commit)
		}
		fmt.Printf("--------------------------------\n")
	}

}
