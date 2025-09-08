package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestGitOpsHistory(t *testing.T) {

	owner := "trivago"
	repo := "hotel-search-web"
	branch := "gitops/advertisers"
	fileName := "manifests/api/prod"
	pathFilter := func(path string) bool {
		return strings.HasPrefix(path, fileName)
	}
	since := time.Now().AddDate(0, -1, 0)

	auth := &http.BasicAuth{Username: "git", Password: os.Getenv("GITHUB_TOKEN")}

	url := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	r, err := cloneRepoBranch(url, branch, 0, auth)
	if err != nil {
		t.Fatalf("Failed to clone repo: %v", err)
	}

	commits, err := r.Log(&git.LogOptions{
		All:        true,
		Since:      &since,
		PathFilter: pathFilter,
	})
	if err != nil {
		t.Fatalf("Failed to get commits: %v", err)
	}

	commits.ForEach(func(c *object.Commit) error {
		message := c.Message

		// Extract SHA from message if it follows the pattern: repo@SHA
		repoSHAPattern := regexp.MustCompile(`[\w-]+/[\w-]+@([0-9a-f]{40})`)
		matches := repoSHAPattern.FindStringSubmatch(message)
		var extractedSHA string
		if len(matches) > 1 {
			extractedSHA = matches[1]
		}

		fmt.Printf("SHA: %s, RemoteSHA: %s, Author: %s, Email: %s, Date: %s\n", c.Hash, extractedSHA, c.Author.Name, c.Author.Email, c.Author.When)
		fmt.Printf("Parents: %v\n", c.ParentHashes)
		return nil
	})

}
