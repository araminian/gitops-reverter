package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func main() {

	desiredCommitHashFlag := flag.String("desiredCommitHash", "", "The Desired Commit Hash to revert gitops branches to its state")
	ownerFlag := flag.String("owner", "trivago", "The Owner of the GitHub repository")
	repoFlag := flag.String("repo", "hsw-fork", "The Name of the GitHub repository")
	pathFlag := flag.String("path", "manifests/api/prod", "The Path within the gitops branches to analyze")
	ignoreBranchesFlag := flag.String("ignoreBranches", "gitops/sink,gitops/infra,gitops/stage,gitops/seo-indexation,gitops/member-data", "The Comma-separated list of gitops branches to ignore")
	sinceFlag := flag.Int("since", 1, "The Number of months ago to get the commits")
	rollbackFlag := flag.Bool("rollback", false, "The Mode to run the program, if true, it will run in rollback mode. Otherwise, it will just print the commits to revert")
	pushFlag := flag.Bool("push", false, "if true, it will push the changes to the remote repository. Otherwise, it will just commit the changes")

	flag.Usage = func() {
		fmt.Printf("\nUsage: %s <desiredCommitHash> <owner> <repo> <path> <Comma-separated list of gitops branches to ignore> <since> <rollback> <push>\n", os.Args[0])
		fmt.Printf("\nEnvironment variables:")
		fmt.Printf("\n  GITHUB_TOKEN     GitHub personal access token (required)")
		fmt.Printf("\n")
		fmt.Printf("\nExample: %s f50d95b53a5d9fdb2a1039b6a86aa180ee1afb3d trivago hotel-search-web manifests/api/prod gitops/skip-branch,gitops/infra 1 true false\n", os.Args[0])
		flag.PrintDefaults()
	}

	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Fatalf("GITHUB_TOKEN environment variable is not set")
	}

	flag.Parse()

	commitHash := *desiredCommitHashFlag
	since := time.Now().AddDate(0, -*sinceFlag, 0)
	owner := *ownerFlag
	repo := *repoFlag
	path := *pathFlag
	basicAuth := &http.BasicAuth{Username: "git", Password: os.Getenv("GITHUB_TOKEN")}
	ignoreBranches := strings.Split(*ignoreBranchesFlag, ",")
	rollbackMode := *rollbackFlag
	pushMode := *pushFlag

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		log.Fatalf("Failed to create github client: %v", err)
	}

	// List all gitops branches
	branches, err := listGitOpsBranches(owner, repo, ignoreBranches)
	if err != nil {
		log.Fatalf("Failed to list gitops branches: %v", err)
	}

	// Get all commits since <since> months ago on master
	commits, err := client.ListCommitsSince(context.Background(), since, "master")
	if err != nil {
		log.Fatalf("Failed to list commits: %v", err)
	}

	masterCommits := processHeadCommits(commits)

	commitGraph, commitsHistory, err := generateCommitGraph(owner, repo, branches, masterCommits, path)
	if err != nil {
		log.Fatalf("Failed to generate commit graph: %v", err)
	}

	log.Printf("Commit Graph:")
	log.Printf("------------------- START COMMIT GRAPH -------------------")
	for _, commit := range commitGraph {
		log.Printf("Commit: %s\n", commit.SHA)
		log.Printf("Parent: %s\n", commit.Parent)
		log.Printf("Date: %v\n", commit.Date)
		log.Printf("GitOps Commits: %v\n", commit.GitOpsCommits)
		log.Printf("--------------------------------\n")
	}
	log.Printf("------------------- END COMMIT GRAPH -------------------")

	rollbackCommits, err := findRollbackCommits(commitGraph, branches, commitHash)
	if err != nil {
		log.Fatalf("Failed to find rollback commits: %v", err)
	}

	log.Printf("Rollback Commits:")
	log.Printf("------------------- START ROLLBACK COMMITS -------------------")
	for branch, commit := range rollbackCommits {
		log.Printf("------------ START BRANCH %s-------------\n", branch)
		log.Printf("Branch: %s\n", branch)
		log.Printf("GitOps Commit: %s\n", commit.GitOpsCommit)
		log.Printf("Head Commit: %s\n", commit.HeadCommit)
		log.Printf("------------ END BRANCH %s-------------\n", branch)
	}
	log.Printf("------------------- END ROLLBACK COMMITS -------------------")

	log.Printf("Finding commits after the gitops commit related to the desired commit")
	commitsAfterRollback, err := findCommitsAfterRollback(rollbackCommits, commitsHistory)
	if err != nil {
		log.Fatalf("Failed to find commits after the gitops commit related to the desired commit: %v", err)
	}

	//  Newest to oldest
	log.Printf("------------------- START ROLLBACK -------------------")
	for branch, commits := range commitsAfterRollback {
		log.Printf("------------ START BRANCH %s-------------\n", branch)
		log.Printf("Branch: %s\n", branch)
		log.Printf("Number of commits to revert: %d\n", len(commits))

		for i, commit := range commits {
			log.Printf("Commit %d: %s\n", i, commit)
		}

		if len(commits) == 0 {
			log.Printf("No commits to revert on branch %s", branch)
			continue
		}

		log.Printf("Reverting %d commits on branch %s", len(commits), branch)
		if !rollbackMode {
			log.Printf("Skipping revert of commits on branch %s, rollbackMode is false", branch)
			continue
		}
		err = revertFromCommitCLI(owner, repo, basicAuth, branch, commits, true, pushMode)
		if err != nil {
			log.Fatalf("Failed to revert commits on branch %s: %v", branch, err)
		}

		log.Printf("------------ END BRANCH %s-------------\n", branch)
	}
	log.Printf("------------------- END ROLLBACK -------------------")

}
