package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
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

type Commit struct {
	SHA             string
	Created         time.Time
	Message         string
	IsDesiredCommit bool
}

// Worker pool implementation
func findCommitOnBranches(url string, auth *http.BasicAuth, branches []string, tier string, service string, commitHash string, since time.Time, workers int) (map[string][]Commit, error) {
	// Set default number of workers if not specified
	if workers <= 0 {
		workers = 4 // Default to 4 workers
	}

	// If we have more workers than branches, limit workers to number of branches
	if workers > len(branches) {
		workers = len(branches)
	}

	var wg sync.WaitGroup
	commitsMutex := sync.Mutex{}
	commits := make(map[string][]Commit)
	errChan := make(chan error, len(branches))

	// Create a channel to distribute work
	branchChan := make(chan string, len(branches))

	// Add all branches to the channel
	for _, branch := range branches {
		branchChan <- branch
	}
	close(branchChan)

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for branch := range branchChan {
				repo, err := cloneRepoBranch(url, branch, 0, auth)
				log.Printf("Cloned repo: %s", branch)
				if err != nil {
					errChan <- fmt.Errorf("error cloning branch %s: %w", branch, err)
					return
				}

				logIter, err := repo.Log(&git.LogOptions{
					Order: git.LogOrderDefault,
					Since: &since,
				})
				if err != nil {
					errChan <- fmt.Errorf("error getting log for branch %s: %w", branch, err)
					return
				}

				branchCommits := []Commit{}
				err = logIter.ForEach(func(commit *object.Commit) error {
					desiredCommitMessage := fmt.Sprintf("%s : new release - %s", service, tier)
					matchAll := []string{
						desiredCommitMessage,
					}

					for _, match := range matchAll {
						if !strings.Contains(commit.Message, match) {
							return nil
						}
					}

					isDesiredCommit := strings.Contains(commit.Message, commitHash)
					branchCommits = append(branchCommits, Commit{
						SHA:             commit.Hash.String(),
						Created:         commit.Committer.When,
						Message:         commit.Message,
						IsDesiredCommit: isDesiredCommit,
					})

					return nil
				})

				if err != nil {
					errChan <- fmt.Errorf("error processing branch %s: %w", branch, err)
					return
				}

				// Safely update the commits map
				if len(branchCommits) > 0 {
					commitsMutex.Lock()
					commits[branch] = branchCommits
					commitsMutex.Unlock()
				}
			}
		}()
	}

	// Wait for all workers to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return commits, nil
}
