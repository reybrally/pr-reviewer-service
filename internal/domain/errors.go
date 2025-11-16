package domain

import "errors"

var (
	ErrTeamExists        = errors.New("team already exists")
	ErrPullRequestExists = errors.New("pull request already exists")
	ErrPullRequestMerged = errors.New("pull request already merged")
	ErrNotAssigned       = errors.New("reviewer is not assigned to this pull request")
	ErrNoCandidate       = errors.New("no active replacement candidate in team")
	ErrNotFound          = errors.New("resource not found")
)
