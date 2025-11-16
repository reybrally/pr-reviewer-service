package domain

import "time"

type UserID string
type TeamName string
type PullRequestID string

type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"
)

type User struct {
	ID       UserID
	Username string
	TeamName TeamName
	IsActive bool
}

type Team struct {
	Name    TeamName
	Members []User
}

type PullRequest struct {
	ID                PullRequestID
	Name              string
	AuthorID          UserID
	Status            PRStatus
	AssignedReviewers []UserID
	CreatedAt         time.Time
	MergedAt          *time.Time
}

type PullRequestShort struct {
	ID       PullRequestID
	Name     string
	AuthorID UserID
	Status   PRStatus
}
