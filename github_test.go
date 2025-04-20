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
