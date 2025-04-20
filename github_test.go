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

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		t.Fatalf("Failed to create github client: %v", err)
	}

	// 17 Apr 2025 , 5PM , CET = GMT + 2
	since := time.Date(2025, 4, 17, 17, 0, 0, 0, time.FixedZone("CET", 2*3600))

	commits, err := client.ListCommitsSince(context.Background(), since)
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
