package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func main() {
	auth := &http.BasicAuth{Username: "git", Password: os.Getenv("GITHUB_TOKEN")}
	commitHash := "c17f5d14da563de7887f79ad8be21e076de45848"
	since := time.Now().AddDate(0, 0, -2)
	workers := 10
	owner := "trivago"
	repo := "hotel-search-web"
	url := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	branch := "master"

	// // Use Git to find the commit history after the specific commit
	// commitsAfterCommit, err := findCommitHistoryAfterSpecificCommit("https://github.com/trivago/hotel-search-web", "master", commitHash, auth, since)
	// if err != nil {
	// 	log.Fatalf("Failed to find commit history after specific commit: %v", err)
	// }

	// for _, c := range commitsAfterCommit {
	// 	log.Printf("Commit: %s", c)
	// }

	// Use Github to find the commit history after the specific commit

	githubClient, err := NewGithubClient(owner, repo)
	if err != nil {
		log.Fatalf("Failed to create Github client: %v", err)
	}

	commitsGithub, err := githubClient.ListCommitsAfterCommit(context.Background(), branch, commitHash, since)
	if err != nil {
		log.Fatalf("Failed to list commits: %v", err)
	}

	commitsAfterDesiredCommit := make([]string, 0)

	for _, c := range commitsGithub {
		commitsAfterDesiredCommit = append(commitsAfterDesiredCommit, c.GetSHA())
	}

	log.Printf("Found %v commits to check", commitsAfterDesiredCommit)

	filter := func(branch string) bool {
		return strings.HasPrefix(branch, "gitops/")
	}

	branches, err := listBranches(url, auth, filter)
	if err != nil {
		log.Fatalf("Failed to list branches: %v", err)
	}

	// for _, branch := range branches {
	// 	log.Printf("Branch: %s", branch)
	// }

	commits, err := findCommitOnBranches(url, auth, branches, "prod", "api", commitHash, commitsAfterDesiredCommit, since, workers)
	if err != nil {
		log.Fatalf("Failed to find commits: %v", err)
	}

	// for branch, commit := range commits {
	// 	log.Printf("Branch: %s", branch)
	// 	for _, c := range commit {
	// 		log.Printf("Commit: %s", c.Message)
	// 		log.Printf("Commit SHA: %s", c.SHA)
	// 		log.Printf("Commit Created: %s", c.Created)
	// 		log.Printf("Commit IsDesiredCommit: %t", c.IsDesiredCommit)
	// 	}
	// 	log.Printf("--------------------------------")
	// }

	revertCommits, err := findRevertSHAs(commits)
	if err != nil {
		log.Fatalf("Failed to find revert commits: %v", err)
	}

	for branch, revertCommit := range revertCommits {
		log.Printf("Branch: %s", branch)
		log.Printf("Revert SHA: %s", revertCommit.SHA)
		log.Printf("Message: %s", revertCommit.Message)
		log.Printf("--------------------------------")
	}

}
