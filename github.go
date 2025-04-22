package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

// ListCommitsSince list all commits since a specific time
func (c *GithubClient) ListCommitsSince(ctx context.Context, since time.Time) ([]*github.RepositoryCommit, error) {
	opts := &github.CommitsListOptions{
		Since:       since,
		ListOptions: github.ListOptions{PerPage: 100},
	}

	allCommits := make([]*github.RepositoryCommit, 0)

	for {
		pageCommits, resp, err := c.client.Repositories.ListCommits(ctx, c.owner, c.repo, opts)
		if err != nil {
			return nil, err
		}

		// This was the bug - we were appending commits to itself instead of to allCommits
		allCommits = append(allCommits, pageCommits...)

		if resp.NextPage == 0 {
			break
		}

		opts.Page = resp.NextPage
	}

	return allCommits, nil
}

// listAllWorkflowsRuns lists all workflow runs for a given repository
func (c *GithubClient) ListAllWorkflowsRuns(ctx context.Context, branch string) ([]*github.WorkflowRun, error) {

	statuses := []string{
		"in_progress",
		"queued",
		"waiting",
		"pending",
	}

	workflowsRunList := make([]*github.WorkflowRun, 0)

	for _, status := range statuses {

		for {
			opts := &github.ListWorkflowRunsOptions{
				Status: status,
				Branch: branch,
				ListOptions: github.ListOptions{
					PerPage: 100,
				},
			}

			workflowRuns, resp, err := c.client.Actions.ListRepositoryWorkflowRuns(
				ctx,
				c.owner,
				c.repo,
				opts,
			)

			if err != nil {
				log.Printf("Error listing workflow runs for status %s: %v", status, err)
				continue
			}

			workflowsRunList = append(workflowsRunList, workflowRuns.WorkflowRuns...)

			if resp.NextPage == 0 {
				break
			}

			opts.Page = resp.NextPage

		}

	}

	return workflowsRunList, nil

}

// ForceCancelWorkflowRun force cancels a workflow run
func (c *GithubClient) ForceCancelWorkflowRun(ctx context.Context, workflowRunID int64) error {

	// use /force-cancel endpoint to cancel a workflow run
	url := fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d/force-cancel", c.owner, c.repo, workflowRunID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(ctx, req, nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("failed to force cancel workflow run: %s", resp.Status)
	}

	return nil
}

// DisableWorkflow disables a workflow
func (c *GithubClient) DisableWorkflow(ctx context.Context, workflowID int64) error {

	resp, err := c.client.Actions.DisableWorkflowByID(ctx, c.owner, c.repo, workflowID)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to disable workflow: %s", resp.Status)
	}

	return nil
}
