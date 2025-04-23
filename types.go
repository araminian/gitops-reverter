package main

import "time"

type HeadCommit struct {
	SHA           string
	Parent        string
	Date          time.Time
	GitOpsCommits map[string]GitOpsCommit
}

type GitOpsCommit struct {
	SHA  string
	Date time.Time
}
