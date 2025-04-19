package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func main() {
	auth := &http.BasicAuth{Username: "git", Password: os.Getenv("GITHUB_TOKEN")}
	// repo, err := cloneRepoBranch("https://github.com/trivago/hotel-search-web", "gitops/advertisers", 0, auth)
	// if err != nil {
	// 	log.Fatalf("Failed to clone repo: %v", err)
	// }

	// matchAll := []string{
	// 	"c17f5d14da563de7887f79ad8be21e076de45848",
	// 	"api : new release - prod",
	// }

	// since := time.Now().AddDate(0, 0, -7)

	// // Get commits using Log with proper error handling
	// logIter, err := repo.Log(&git.LogOptions{
	// 	Order: git.LogOrderDefault,
	// 	Since: &since,
	// })
	// if err != nil {
	// 	log.Fatalf("Failed to get log: %v", err)
	// }

	// logIter.ForEach(func(commit *object.Commit) error {
	// 	for _, match := range matchAll {
	// 		if !strings.Contains(commit.Message, match) {
	// 			return nil
	// 		}
	// 	}

	// 	log.Printf("Commit: %s", commit.Message)
	// 	log.Printf("Commit SHA: %s", commit.Hash.String())

	// 	return nil
	// })

	filter := func(branch string) bool {
		return strings.HasPrefix(branch, "gitops/")
	}

	branches, err := listBranches("https://github.com/trivago/hotel-search-web", auth, filter)
	if err != nil {
		log.Fatalf("Failed to list branches: %v", err)
	}

	// for _, branch := range branches {
	// 	log.Printf("Branch: %s", branch)
	// }

	commits, err := findCommitOnBranches("https://github.com/trivago/hotel-search-web", auth, branches, "prod", "api", "c17f5d14da563de7887f79ad8be21e076de45848", time.Now().AddDate(0, 0, -2))
	if err != nil {
		log.Fatalf("Failed to find commits: %v", err)
	}

	for branch, commit := range commits {
		log.Printf("Branch: %s", branch)
		for _, c := range commit {
			log.Printf("Commit: %s", c.Message)
			log.Printf("Commit SHA: %s", c.SHA)
			log.Printf("Commit Created: %s", c.Created)
			log.Printf("Commit IsDesiredCommit: %t", c.IsDesiredCommit)
		}
		log.Printf("--------------------------------")
	}
}
