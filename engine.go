package main

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v71/github"
)

func processHeadCommits(commits []*github.RepositoryCommit) map[string]*HeadCommit {

	headCommits := make(map[string]*HeadCommit)

	for _, commit := range commits {
		headCommits[commit.GetSHA()] = &HeadCommit{
			SHA:           commit.GetSHA(),
			Parent:        commit.Parents[0].GetSHA(),
			Date:          commit.GetCommit().GetAuthor().GetDate().Local(),
			GitOpsCommits: make(map[string]GitOpsCommit),
		}
	}

	return headCommits
}

func listGitOpsBranches(owner, repo string, ignore []string) ([]string, error) {

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		return nil, err
	}

	filter := func(branch string) bool {
		return strings.HasPrefix(branch, "gitops/") && !slices.Contains(ignore, branch)
	}

	gitopsBranches, err := client.ListBranches(context.Background(), filter, true)
	if err != nil {
		return nil, err
	}

	branches := make([]string, len(gitopsBranches))
	for i, branch := range gitopsBranches {
		branches[i] = branch.GetName()
	}

	return branches, nil
}

func generateCommitGraph(owner, repo string, gitopsBranches []string, headCommits map[string]*HeadCommit, path string) (commitsGraph map[string]*HeadCommit, err error) {

	client, err := NewGithubClient(owner, repo)
	if err != nil {
		return nil, err
	}

	commitsGraph = make(map[string]*HeadCommit, len(headCommits))
	for sha, commit := range headCommits {
		commitsGraph[sha] = commit
	}

	since := time.Now().AddDate(0, -1, 0)

	for _, branch := range gitopsBranches {
		branchCommits, err := client.ListCommitsSinceOnPath(context.Background(), since, branch, path)
		if err != nil {
			continue
		}

		for _, commit := range branchCommits {

			message := commit.GetCommit().GetMessage()
			repoSHAPattern := regexp.MustCompile(`[\w-]+/[\w-]+@([0-9a-f]{40})`)
			matches := repoSHAPattern.FindStringSubmatch(message)
			var extractedSHA string
			if len(matches) > 1 {
				extractedSHA = matches[1]
			}

			if extractedSHA == "" {
				continue
			}

			fmt.Printf("Branch: %s\n", branch)
			fmt.Printf("Extracted SHA: %s\n", extractedSHA)

			// Check if the commit is in the head commits map
			if _, ok := commitsGraph[extractedSHA]; !ok {
				continue
			}

			commitsGraph[extractedSHA].GitOpsCommits[branch] = GitOpsCommit{
				SHA:  commit.GetSHA(),
				Date: commit.GetCommit().GetAuthor().GetDate().Local(),
			}

		}

	}

	return commitsGraph, nil
}

// findRollbackCommits finds rollback commits for a given head commit on gitops branches
func findRollbackCommits(commitsGraph map[string]*HeadCommit, gitopsBranches []string, candidateCommit string) (map[string]RollbackCommit, error) {

	rollbackCommits := make(map[string]RollbackCommit, len(gitopsBranches))

	branchesToCheck := make(map[string]bool, len(gitopsBranches))
	for _, b := range gitopsBranches {
		branchesToCheck[b] = true
	}

	if _, ok := commitsGraph[candidateCommit]; !ok {
		return nil, fmt.Errorf("candidate commit not found in commits graph")
	}

	commitToCheck := candidateCommit
	for {

		if len(branchesToCheck) == 0 {
			break
		}

		fmt.Printf("Checking commit: %s\n", commitToCheck)
		fmt.Printf("Branches to check: %v\n", branchesToCheck)

		gitopsCommits := commitsGraph[commitToCheck].GitOpsCommits
		for b, c := range gitopsCommits {
			if !branchesToCheck[b] {
				continue
			}

			// Remove the branch from the branches to check
			delete(branchesToCheck, b)

			rollbackCommits[b] = RollbackCommit{
				GitOpsCommit: c.SHA,
				HeadCommit:   commitToCheck,
			}
		}

		commitToCheck = commitsGraph[commitToCheck].Parent
	}

	return rollbackCommits, nil
}
