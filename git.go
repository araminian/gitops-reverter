package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
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

func findCommitOnBranches(url string, auth *http.BasicAuth, branches []string, tier string, service string, commitHash string, afterCommits []string, since time.Time, workers int) (map[string][]Commit, error) {
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

					// If the commit is not the desired commit, check if there is a commit after the desired commit
					if !isDesiredCommit {
						for i := len(afterCommits) - 1; i >= 0; i-- {
							if strings.Contains(commit.Message, afterCommits[i]) {
								isDesiredCommit = true
								break
							}
						}
					}

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

type RevertCommitInfo struct {
	Branch  string
	SHA     string
	Message string
}

func findRevertSHAs(commits map[string][]Commit) (map[string]RevertCommitInfo, error) {

	revertCommits := make(map[string]RevertCommitInfo)

	for branch, commits := range commits {
		for _, commit := range commits {
			if commit.IsDesiredCommit {
				revertCommits[branch] = RevertCommitInfo{
					Branch:  branch,
					SHA:     commit.SHA,
					Message: commit.Message,
				}
			}
		}
	}

	return revertCommits, nil
}

// findCommitHistoryAfterSpecificCommit finds lists of commits that happened after a specific commit, it's git based
func findCommitHistoryAfterSpecificCommit(url, branch, commitHash string, auth *http.BasicAuth, since time.Time) ([]string, error) {

	storage := memory.NewStorage()

	// Initialize a new remote
	remote := git.NewRemote(storage, &config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	// List references to get the branch head
	refs, err := remote.List(&git.ListOptions{Auth: auth})
	if err != nil {
		log.Printf("Error listing remote references: %v", err)
		return nil, err
	}

	// Find the reference for our branch
	var headRef *plumbing.Reference
	branchRefName := plumbing.NewBranchReferenceName(branch)
	for _, ref := range refs {
		if ref.Name() == branchRefName {
			headRef = ref
			break
		}
	}

	if headRef == nil {
		return nil, fmt.Errorf("branch %s not found", branch)
	}

	// Only fetch the relevant data using the reference
	fetchOpts := &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(headRef.Name() + ":" + headRef.Name())},
		Auth:       auth,
		Depth:      100,
	}

	repo, err := git.Init(storage, nil)
	if err != nil {
		log.Printf("Error initializing repo: %v", err)
		return nil, err
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{url},
	})

	if err != nil {
		log.Printf("Error creating remote: %v", err)
		return nil, err
	}

	// Fetch the commits
	err = repo.Fetch(fetchOpts)
	if err != nil && err != git.NoErrAlreadyUpToDate {
		log.Printf("Error fetching: %v", err)
		return nil, err
	}

	// Get the reference to the fetched branch
	ref, err := repo.Reference(headRef.Name(), true)
	if err != nil {
		log.Printf("Error getting reference: %v", err)
		return nil, err
	}

	headCommit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		log.Printf("Error getting head commit: %v", err)
		return nil, err
	}

	iter := object.NewCommitPreorderIter(headCommit, nil, nil)

	commits := []string{}
	found := false

	err = iter.ForEach(func(commit *object.Commit) error {
		if commit.Committer.When.Before(since) {
			return fmt.Errorf("reached commits older than the specified time")
		}

		if commit.Hash.String() == commitHash {
			found = true
			commits = append(commits, commit.Hash.String())
			return nil
		}

		if !found {
			commits = append(commits, commit.Hash.String())
		}

		return nil
	})

	if err != nil && err.Error() != "reached commits older than the specified time" && !found {
		log.Printf("Warning: target commit %s not found within fetched depth", commitHash)
	}

	return commits, nil
}

func RevertCommit(url string, auth *http.BasicAuth, branch string, commitSHA string, force bool, author string, email string) error {
	// Use in-memory storage and filesystem
	storage := memory.NewStorage()
	fs := memfs.New()

	// Clone the repository with the specified branch
	refName := plumbing.NewBranchReferenceName(branch)
	repo, err := git.Clone(
		storage,
		fs,
		&git.CloneOptions{
			URL:           url,
			ReferenceName: refName,
			SingleBranch:  true,
			Auth:          auth,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Get the worktree
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the commit to revert
	commitToRevert, err := repo.CommitObject(plumbing.NewHash(commitSHA))
	if err != nil {
		return fmt.Errorf("failed to get commit object %s: %w", commitSHA, err)
	}

	// Get the commit message to use in the revert
	revertMessage := fmt.Sprintf("Revert \"%s\"\n\nThis reverts commit %s",
		strings.Split(commitToRevert.Message, "\n")[0], commitSHA)

	// Since we can't run an external git command on an in-memory filesystem,
	// we need to manually perform the revert

	// Get the parent commit of the commit to revert
	if commitToRevert.NumParents() == 0 {
		return fmt.Errorf("cannot revert a commit with no parents")
	}

	parentCommit, err := commitToRevert.Parent(0)
	if err != nil {
		return fmt.Errorf("failed to get parent commit: %w", err)
	}

	// Get the trees for comparison
	commitTree, err := commitToRevert.Tree()
	if err != nil {
		return fmt.Errorf("failed to get commit tree: %w", err)
	}

	parentTree, err := parentCommit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get parent tree: %w", err)
	}

	// Get changes between the commit to revert and its parent
	changes, err := commitTree.Diff(parentTree)
	if err != nil {
		return fmt.Errorf("failed to get changes: %w", err)
	}

	// Apply the inverse of the changes (this is what a revert does)
	for _, change := range changes {
		// We check the From and To fields to determine the type of change
		fromEmpty := change.From.Name == ""
		toEmpty := change.To.Name == ""

		if fromEmpty && !toEmpty {
			// This was a file addition in the original commit, we need to remove it
			_, err = wt.Remove(change.To.Name)
			if err != nil {
				return fmt.Errorf("failed to remove file %s: %w", change.To.Name, err)
			}
		} else if !fromEmpty && toEmpty {
			// This was a file deletion in the original commit, we need to restore it
			fromFile, err := parentTree.File(change.From.Name)
			if err != nil {
				return fmt.Errorf("failed to get file %s from parent tree: %w", change.From.Name, err)
			}

			content, err := fromFile.Contents()
			if err != nil {
				return fmt.Errorf("failed to get contents of file %s: %w", change.From.Name, err)
			}

			// Create directories if they don't exist
			dir := change.From.Name
			lastSlash := strings.LastIndex(dir, "/")
			if lastSlash > 0 {
				dir = dir[:lastSlash]
				err = wt.Filesystem.MkdirAll(dir, 0755)
				if err != nil {
					return fmt.Errorf("failed to create directories for %s: %w", change.From.Name, err)
				}
			}

			// Create the file in the worktree
			f, err := wt.Filesystem.Create(change.From.Name)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", change.From.Name, err)
			}

			_, err = f.Write([]byte(content))
			if err != nil {
				f.Close()
				return fmt.Errorf("failed to write to file %s: %w", change.From.Name, err)
			}

			err = f.Close()
			if err != nil {
				return fmt.Errorf("failed to close file %s: %w", change.From.Name, err)
			}

			_, err = wt.Add(change.From.Name)
			if err != nil {
				return fmt.Errorf("failed to add file %s to index: %w", change.From.Name, err)
			}
		} else {
			// This was a file modification in the original commit, we need to restore the parent version
			fromFile, err := parentTree.File(change.From.Name)
			if err != nil {
				return fmt.Errorf("failed to get file %s from parent tree: %w", change.From.Name, err)
			}

			content, err := fromFile.Contents()
			if err != nil {
				return fmt.Errorf("failed to get contents of file %s: %w", change.From.Name, err)
			}

			// Update the file in the worktree
			f, err := wt.Filesystem.Create(change.From.Name)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", change.From.Name, err)
			}

			_, err = f.Write([]byte(content))
			if err != nil {
				f.Close()
				return fmt.Errorf("failed to write to file %s: %w", change.From.Name, err)
			}

			err = f.Close()
			if err != nil {
				return fmt.Errorf("failed to close file %s: %w", change.From.Name, err)
			}

			_, err = wt.Add(change.From.Name)
			if err != nil {
				return fmt.Errorf("failed to add file %s to index: %w", change.From.Name, err)
			}
		}
	}

	// Commit the revert
	_, err = wt.Commit(revertMessage, &git.CommitOptions{
		Author: &object.Signature{
			Name:  author,
			Email: email,
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to commit revert: %w", err)
	}

	// Push the changes
	err = repo.Push(&git.PushOptions{
		Auth:  auth,
		Force: force,
	})
	if err != nil {
		return fmt.Errorf("failed to push revert: %w", err)
	}

	return nil
}
