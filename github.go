package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/go-github/v71/github"
)

type GithubClient struct {
	client *github.Client
	owner  string
	repo   string
}

func NewGithubClient(owner, repo string) (*GithubClient, error) {

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	return &GithubClient{
		client: github.NewClient(nil).WithAuthToken(token),
		owner:  owner,
		repo:   repo,
	}, nil
}

// ListCommitsAfterCommit lists all commits after a specific commit
func (c *GithubClient) ListCommitsAfterCommit(ctx context.Context, branch, commitHash string, since time.Time) ([]*github.RepositoryCommit, error) {

	opts := &github.CommitsListOptions{
		Since: since,
	}
	commits, _, err := c.client.Repositories.ListCommits(ctx, c.owner, c.repo, opts)
	if err != nil {
		return nil, err
	}

	desiredCommits := make([]*github.RepositoryCommit, 0)
	isFound := false

	for _, commit := range commits {
		if commit.GetSHA() == commitHash {
			isFound = true
			break
		}
		desiredCommits = append(desiredCommits, commit)
	}

	if !isFound {
		log.Printf("Commit %s not found in branch %s", commitHash, branch)
	}

	return desiredCommits, nil
}

// listCommitsSince list all commits since a specific time
func (c *GithubClient) ListCommitsSince(ctx context.Context, since time.Time) ([]*github.RepositoryCommit, error) {
	opts := &github.CommitsListOptions{
		Since: since,
	}

	commits, _, err := c.client.Repositories.ListCommits(ctx, c.owner, c.repo, opts)
	if err != nil {
		return nil, err
	}

	return commits, nil
}
