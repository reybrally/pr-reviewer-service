package domain

import (
	"context"
	"time"
)

type TeamRepository interface {
	CreateTeam(ctx context.Context, name TeamName) error
	GetTeam(ctx context.Context, name TeamName) (Team, error)
	TeamExists(ctx context.Context, name TeamName) (bool, error)
}

type UserRepository interface {
	UpsertUsers(ctx context.Context, users []User) error
	GetByID(ctx context.Context, id UserID) (User, error)
	SetIsActive(ctx context.Context, id UserID, isActive bool) (User, error)
	ListActiveByTeam(ctx context.Context, teamName TeamName) ([]User, error)
}

type PullRequestRepository interface {
	Create(ctx context.Context, pr PullRequest) error
	Exists(ctx context.Context, id PullRequestID) (bool, error)
	Get(ctx context.Context, id PullRequestID) (PullRequest, error)
	MarkMerged(ctx context.Context, id PullRequestID, mergedAt time.Time) error
	ReplaceReviewer(ctx context.Context, prID PullRequestID, oldUserID, newUserID UserID) error
	ListByReviewer(ctx context.Context, reviewerID UserID) ([]PullRequestShort, error)
	StatsAssignmentsByUser(ctx context.Context) (map[UserID]int, error)
}
