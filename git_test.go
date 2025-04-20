package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestRevertCommit(t *testing.T) {

	owner := "araminian"
	repo := "gitops-k8s"
	branch := "gitops"

	auth := &http.BasicAuth{Username: "git", Password: os.Getenv("GITHUB_TOKEN")}

	url := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	commitToRevert := "71e3272f60dfc24e86034471ae1072944e0a379d"

	author := "araminian"
	email := "rmin.aminian@gmail.com"

	err := RevertCommit(url, auth, branch, commitToRevert, true, author, email)
	if err != nil {
		t.Fatalf("Failed to revert commit: %v", err)
	}

}
