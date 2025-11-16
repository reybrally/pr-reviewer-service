package service

import (
	"context"
	"errors"
	"math/rand"
	"pr-reviewer-service/internal/domain"
	"time"
)

type PRService struct {
	Users domain.UserRepository
	Prs   domain.PullRequestRepository
	Rand  *rand.Rand
}

func NewPRService(users domain.UserRepository, prs domain.PullRequestRepository) *PRService {
	src := rand.NewSource(time.Now().UnixNano())
	return &PRService{
		Users: users,
		Prs:   prs,
		Rand:  rand.New(src),
	}
}

func (s *PRService) Create(ctx context.Context, id domain.PullRequestID, name string, authorID domain.UserID) (domain.PullRequest, error) {
	exists, err := s.Prs.Exists(ctx, id)
	if err != nil {
		return domain.PullRequest{}, err
	}
	if exists {
		return domain.PullRequest{}, domain.ErrPullRequestExists
	}

	author, err := s.Users.GetByID(ctx, authorID)
	if err != nil {
		return domain.PullRequest{}, err
	}

	candidates, err := s.Users.ListActiveByTeam(ctx, author.TeamName)
	if err != nil {
		return domain.PullRequest{}, err
	}

	var filtered []domain.User
	for _, u := range candidates {
		if u.ID == author.ID {
			continue
		}
		filtered = append(filtered, u)
	}

	assigned := pickRandomReviewers(filtered, 2, s.Rand)

	now := time.Now().UTC()
	pr := domain.PullRequest{
		ID:                id,
		Name:              name,
		AuthorID:          authorID,
		Status:            domain.PRStatusOpen,
		AssignedReviewers: assigned,
		CreatedAt:         now,
		MergedAt:          nil,
	}

	if err := s.Prs.Create(ctx, pr); err != nil {
		return domain.PullRequest{}, err
	}

	return pr, nil
}

func (s *PRService) Merge(ctx context.Context, id domain.PullRequestID) (domain.PullRequest, error) {
	pr, err := s.Prs.Get(ctx, id)
	if err != nil {
		return domain.PullRequest{}, err
	}

	if pr.Status == domain.PRStatusMerged {
		return pr, nil
	}

	now := time.Now().UTC()
	if err := s.Prs.MarkMerged(ctx, id, now); err != nil {
		return domain.PullRequest{}, err
	}

	pr.Status = domain.PRStatusMerged
	pr.MergedAt = &now

	return pr, nil
}

func (s *PRService) Reassign(ctx context.Context, prID domain.PullRequestID, oldUserID domain.UserID) (domain.PullRequest, domain.UserID, error) {
	pr, err := s.Prs.Get(ctx, prID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	if pr.Status == domain.PRStatusMerged {
		return domain.PullRequest{}, "", domain.ErrPullRequestMerged
	}

	found := false
	for _, r := range pr.AssignedReviewers {
		if r == oldUserID {
			found = true
			break
		}
	}
	if !found {
		return domain.PullRequest{}, "", domain.ErrNotAssigned
	}

	oldUser, err := s.Users.GetByID(ctx, oldUserID)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	candidates, err := s.Users.ListActiveByTeam(ctx, oldUser.TeamName)
	if err != nil {
		return domain.PullRequest{}, "", err
	}

	assignedSet := make(map[domain.UserID]struct{}, len(pr.AssignedReviewers))
	for _, r := range pr.AssignedReviewers {
		assignedSet[r] = struct{}{}
	}

	var filtered []domain.User
	for _, u := range candidates {
		if u.ID == oldUserID {
			continue
		}
		if u.ID == pr.AuthorID {
			continue
		}
		if _, ok := assignedSet[u.ID]; ok {
			continue
		}
		filtered = append(filtered, u)
	}

	if len(filtered) == 0 {
		return domain.PullRequest{}, "", domain.ErrNoCandidate
	}

	newReviewer := pickRandomReviewers(filtered, 1, s.Rand)[0]

	if err := s.Prs.ReplaceReviewer(ctx, prID, oldUserID, newReviewer); err != nil {
		return domain.PullRequest{}, "", err
	}

	for i, r := range pr.AssignedReviewers {
		if r == oldUserID {
			pr.AssignedReviewers[i] = newReviewer
			break
		}
	}

	return pr, newReviewer, nil
}

func (s *PRService) ListByReviewer(ctx context.Context, reviewerID domain.UserID) ([]domain.PullRequestShort, error) {
	return s.Prs.ListByReviewer(ctx, reviewerID)
}

func (s *PRService) StatsAssignmentsByUser(ctx context.Context) (map[domain.UserID]int, error) {
	return s.Prs.StatsAssignmentsByUser(ctx)
}

func (s *PRService) BulkDeactivateAndReassign(
	ctx context.Context,
	userIDs []domain.UserID,
) error {
	for _, uid := range userIDs {
		_, err := s.Users.GetByID(ctx, uid)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				continue
			}
			return err
		}

		if _, err := s.Users.SetIsActive(ctx, uid, false); err != nil {
			return err
		}

		prs, err := s.Prs.ListByReviewer(ctx, uid)
		if err != nil {
			return err
		}

		for _, pr := range prs {
			if pr.Status != domain.PRStatusOpen {
				continue
			}

			_, _, err := s.Reassign(ctx, pr.ID, uid)
			if err != nil {
				if errors.Is(err, domain.ErrNoCandidate) || errors.Is(err, domain.ErrPullRequestMerged) {
					continue
				}
				return err
			}
		}
	}

	return nil
}
