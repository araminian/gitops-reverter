package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
)

func cloneRepoBranch(url string, branch string, depth int, auth *http.BasicAuth) (*git.Repository, error) {
	refName := plumbing.NewBranchReferenceName(branch)
	repo, err := git.Clone(
		memory.NewStorage(),
		nil,
		&git.CloneOptions{
			URL:           url,
			ReferenceName: refName,
			SingleBranch:  true,
			Depth:         depth,
			Tags:          git.NoTags,
			Auth:          auth,
		},
	)
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func listBranches(url string, auth *http.BasicAuth, filter func(string) bool) ([]string, error) {
	storage := memory.NewStorage()

	remote := git.NewRemote(storage, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	refs, err := remote.List(&git.ListOptions{Auth: auth})
	if err != nil {
		return nil, err
	}

	branches := []string{}
	refPrefix := "refs/heads/"

	for _, ref := range refs {
		refName := ref.Name().String()
		if strings.HasPrefix(refName, refPrefix) {
			branchName := refName[len(refPrefix):]
			if filter(branchName) {
				branches = append(branches, branchName)
			}
		}
	}

	return branches, nil
}

func findCommitOnBranches(url string, auth *http.BasicAuth, branches []string, tier string, service string, commitHash string, since time.Time) (map[string]string, error) {

	commits := make(map[string]string)

	for _, branch := range branches {
		repo, err := cloneRepoBranch(url, branch, 0, auth)
		log.Printf("Cloned repo: %s", branch)
		if err != nil {
			return nil, err
		}

		logIter, err := repo.Log(&git.LogOptions{
			Order: git.LogOrderDefault,
			Since: &since,
		})
		if err != nil {
			return nil, err
		}

		err = logIter.ForEach(func(commit *object.Commit) error {

			desiredCommitMessage := fmt.Sprintf("%s : new release - %s", service, tier)
			matchAll := []string{
				commitHash,
				desiredCommitMessage,
			}

			for _, match := range matchAll {
				if !strings.Contains(commit.Message, match) {
					return nil
				}
			}

			commits[branch] = commit.Hash.String()

			return nil
		})

		if err != nil {
			return nil, err
		}

	}

	return commits, nil

}
