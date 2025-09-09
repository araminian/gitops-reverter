package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

var worktreeLock sync.Mutex

func createWorktree(repoPath string, branch string, branchWorktreePath string) error {

	worktreeLock.Lock()
	defer worktreeLock.Unlock()

	cmd := exec.Command("git", "worktree", "add", branchWorktreePath, branch)
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w, output: %s", err, output)
	}

	return nil
}

func configureGit() error {

	if os.Getenv("CI") == "true" {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			panic("GITHUB_TOKEN environment variable is not set")
		}
		REPL_URL := fmt.Sprintf("https://x-access-token:%s@github.com", os.Getenv("GITHUB_TOKEN"))
		log.Printf("CI variable is set, configuring git to use the token")
		gitConfigSSH := exec.Command("git", "config", "--global", "--add",
			fmt.Sprintf("url.%s.insteadOf", REPL_URL), "ssh://git@github")
		gitConfigSSH.Run()

		gitConfigHTTPS := exec.Command("git", "config", "--global", "--add",
			fmt.Sprintf("url.%s.insteadOf", REPL_URL), "https://github")
		gitConfigHTTPS.Run()

		gitConfigGit := exec.Command("git", "config", "--global", "--add",
			fmt.Sprintf("url.%s.insteadOf", REPL_URL), "git@github")
		gitConfigGit.Run()
	} else {
		log.Printf("CI variable is not set, skipping git configuration")
	}

	return nil
}

func cloneRepositoryCLI(owner, repoName string) (string, error) {

	url := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)

	repoDir, err := os.MkdirTemp("", "git-revert-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	err = configureGit()
	if err != nil {
		return "", fmt.Errorf("failed to configure git: %w", err)
	}
	cloneCmd := exec.Command("git", "clone", url, repoDir)
	cloneCmd.Dir = repoDir
	err = cloneCmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	return repoDir, nil

}

// revertFromCommitCLI reverts multiple commits in a single command
func revertFromCommitCLI(repoDir string, branch string, commits []string, force bool, pushMode bool) error {
	// Check if commits slice is empty
	if len(commits) == 0 {
		return fmt.Errorf("no commits provided to revert")
	}

	err := configureGit()
	if err != nil {
		return fmt.Errorf("failed to configure git: %w", err)
	}

	branchDirName := strings.ReplaceAll(branch, "/", "-")

	// Create a temporary directory to clone the repository
	branchRootDir, err := os.MkdirTemp("", fmt.Sprintf("git-revert-%s-*-*", branchDirName))
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(branchRootDir) // Clean up when we're done

	// Create a worktree for the branch
	err = createWorktree(repoDir, branch, branchRootDir)
	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Add author information for git commands
	// Execute git revert command using CLI for all commits at once
	revertArgs := []string{"revert", "--no-edit"}

	if force {
		revertArgs = append(revertArgs, "--no-gpg-sign")
	}

	// Add all commits to revert in a single command (from newest to oldest)
	revertArgs = append(revertArgs, commits...)

	// Run revert with a timeout and disable any interaction/editor prompts
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	revertCmd := exec.CommandContext(ctx, "git", revertArgs...)
	log.Printf("Revert Command: %v", revertCmd.String())
	revertCmd.Dir = branchRootDir
	revertCmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_MERGE_AUTOEDIT=no",
		"GIT_EDITOR=true",
	)
	revertOutput, err := revertCmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("git revert timed out after 10m: %s", revertOutput)
	}

	if err != nil {
		return fmt.Errorf("failed to revert commits: %s, %w", revertOutput, err)
	}

	// Make sure all new files are added
	gitAddCmd := exec.CommandContext(ctx, "git", "add", ".")
	gitAddCmd.Dir = branchRootDir
	gitAddOutput, err := gitAddCmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("git add timed out after 10m: %s", gitAddOutput)
	}
	if err != nil {
		return fmt.Errorf("failed to add files to index: %s, %w", gitAddOutput, err)
	}

	// Push the changes back using go-git
	if !pushMode {
		log.Printf("Skipping push of changes to remote repository, pushMode is false")
		return nil
	}
	pushCmd := exec.CommandContext(ctx, "git", "push", "origin", branch)
	pushCmd.Dir = branchRootDir
	pushOutput, err := pushCmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("git push timed out after 10m: %s", pushOutput)
	}
	if err != nil {
		return fmt.Errorf("failed to push changes: %s, %w", pushOutput, err)
	}

	return nil
}
