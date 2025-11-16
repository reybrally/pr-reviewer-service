package service

import (
	"context"
	"math/rand"
	"pr-reviewer-service/internal/domain"
	"testing"
	"time"
)

type fakeUserRepo struct {
	users map[domain.UserID]domain.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users: make(map[domain.UserID]domain.User),
	}
}

func (r *fakeUserRepo) UpsertUsers(ctx context.Context, users []domain.User) error {
	for _, u := range users {
		r.users[u.ID] = u
	}
	return nil
}

func (r *fakeUserRepo) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}
func (r *fakePRRepo) StatsAssignmentsByUser(
	ctx context.Context,
) (map[domain.UserID]int, error) {
	return map[domain.UserID]int{}, nil
}

func (r *fakeUserRepo) SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	u.IsActive = isActive
	r.users[id] = u
	return u, nil
}

func (r *fakeUserRepo) ListActiveByTeam(ctx context.Context, teamName domain.TeamName) ([]domain.User, error) {
	var res []domain.User
	for _, u := range r.users {
		if u.TeamName == teamName && u.IsActive {
			res = append(res, u)
		}
	}
	return res, nil
}

type fakePRRepo struct {
	prs map[domain.PullRequestID]domain.PullRequest
}

func newFakePRRepo() *fakePRRepo {
	return &fakePRRepo{
		prs: make(map[domain.PullRequestID]domain.PullRequest),
	}
}

func (r *fakePRRepo) Create(ctx context.Context, pr domain.PullRequest) error {
	r.prs[pr.ID] = pr
	return nil
}

func (r *fakePRRepo) Exists(ctx context.Context, id domain.PullRequestID) (bool, error) {
	_, ok := r.prs[id]
	return ok, nil
}

func (r *fakePRRepo) Get(ctx context.Context, id domain.PullRequestID) (domain.PullRequest, error) {
	pr, ok := r.prs[id]
	if !ok {
		return domain.PullRequest{}, domain.ErrNotFound
	}
	return pr, nil
}

func (r *fakePRRepo) MarkMerged(ctx context.Context, id domain.PullRequestID, mergedAt time.Time) error {
	pr, ok := r.prs[id]
	if !ok {
		return domain.ErrNotFound
	}
	pr.Status = domain.PRStatusMerged
	pr.MergedAt = &mergedAt
	r.prs[id] = pr
	return nil
}

func (r *fakePRRepo) ReplaceReviewer(ctx context.Context, prID domain.PullRequestID, oldUserID, newUserID domain.UserID) error {
	pr, ok := r.prs[prID]
	if !ok {
		return domain.ErrNotFound
	}
	for i, rID := range pr.AssignedReviewers {
		if rID == oldUserID {
			pr.AssignedReviewers[i] = newUserID
			break
		}
	}
	r.prs[prID] = pr
	return nil
}

func (r *fakePRRepo) ListByReviewer(ctx context.Context, reviewerID domain.UserID) ([]domain.PullRequestShort, error) {
	var res []domain.PullRequestShort
	for _, pr := range r.prs {
		for _, rID := range pr.AssignedReviewers {
			if rID == reviewerID {
				res = append(res, domain.PullRequestShort{
					ID:       pr.ID,
					Name:     pr.Name,
					AuthorID: pr.AuthorID,
					Status:   pr.Status,
				})
				break
			}
		}
	}
	return res, nil
}

func TestPRService_Create_AssignsZeroOneTwoReviewers(t *testing.T) {
	ctx := context.Background()

	team := domain.TeamName("backend")
	authorID := domain.UserID("u1")

	makeUsers := func(users []domain.User) *fakeUserRepo {
		repo := newFakeUserRepo()
		for _, u := range users {
			repo.users[u.ID] = u
		}
		return repo
	}

	t.Run("no available candidates -> 0 reviewers", func(t *testing.T) {
		usersRepo := makeUsers([]domain.User{
			{ID: authorID, Username: "Alice", TeamName: team, IsActive: true},
		})
		prRepo := newFakePRRepo()

		svc := &PRService{
			Users: usersRepo,
			Prs:   prRepo,
			Rand:  rand.New(rand.NewSource(1)),
		}

		pr, err := svc.Create(ctx, domain.PullRequestID("pr0"), "PR 0", authorID)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if len(pr.AssignedReviewers) != 0 {
			t.Fatalf("expected 0 reviewers, got %d", len(pr.AssignedReviewers))
		}
	})

	t.Run("one candidate -> 1 reviewer", func(t *testing.T) {
		usersRepo := makeUsers([]domain.User{
			{ID: authorID, Username: "Alice", TeamName: team, IsActive: true},
			{ID: "u2", Username: "Bob", TeamName: team, IsActive: true},
		})
		prRepo := newFakePRRepo()

		svc := &PRService{
			Users: usersRepo,
			Prs:   prRepo,
			Rand:  rand.New(rand.NewSource(2)),
		}

		pr, err := svc.Create(ctx, domain.PullRequestID("pr1"), "PR 1", authorID)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if len(pr.AssignedReviewers) != 1 {
			t.Fatalf("expected 1 reviewer, got %d", len(pr.AssignedReviewers))
		}
		if pr.AssignedReviewers[0] == authorID {
			t.Fatalf("author must not be assigned as reviewer")
		}
	})

	t.Run("many candidates -> exactly 2 reviewers, all valid", func(t *testing.T) {
		usersRepo := makeUsers([]domain.User{
			{ID: authorID, Username: "Alice", TeamName: team, IsActive: true},
			{ID: "u2", Username: "Bob", TeamName: team, IsActive: true},
			{ID: "u3", Username: "Carol", TeamName: team, IsActive: true},
			{ID: "u4", Username: "Dave", TeamName: team, IsActive: true},
			{ID: "u5", Username: "Eve", TeamName: "other", IsActive: true},
			{ID: "u6", Username: "Frank", TeamName: team, IsActive: false},
		})
		prRepo := newFakePRRepo()

		svc := &PRService{
			Users: usersRepo,
			Prs:   prRepo,
			Rand:  rand.New(rand.NewSource(3)),
		}

		pr, err := svc.Create(ctx, domain.PullRequestID("pr2"), "PR 2", authorID)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if len(pr.AssignedReviewers) != 2 {
			t.Fatalf("expected 2 reviewers, got %d", len(pr.AssignedReviewers))
		}

		seen := map[domain.UserID]struct{}{}
		for _, rID := range pr.AssignedReviewers {
			if rID == authorID {
				t.Fatalf("author must not be assigned as reviewer")
			}
			u := usersRepo.users[rID]
			if u.TeamName != team {
				t.Fatalf("reviewer must be from same team, got team %s", u.TeamName)
			}
			if !u.IsActive {
				t.Fatalf("reviewer must be active")
			}
			if _, ok := seen[rID]; ok {
				t.Fatalf("reviewers must be distinct")
			}
			seen[rID] = struct{}{}
		}
	})
}

func TestPRService_Merge_Idempotent(t *testing.T) {
	ctx := context.Background()

	usersRepo := newFakeUserRepo()
	prRepo := newFakePRRepo()

	id := domain.PullRequestID("pr-merge")
	now := time.Now().UTC()

	prRepo.prs[id] = domain.PullRequest{
		ID:                id,
		Name:              "PR merge",
		AuthorID:          domain.UserID("u1"),
		Status:            domain.PRStatusOpen,
		AssignedReviewers: []domain.UserID{"u2"},
		CreatedAt:         now,
		MergedAt:          nil,
	}

	svc := &PRService{
		Users: usersRepo,
		Prs:   prRepo,
		Rand:  rand.New(rand.NewSource(1)),
	}

	pr1, err := svc.Merge(ctx, id)
	if err != nil {
		t.Fatalf("Merge(1) returned error: %v", err)
	}
	if pr1.Status != domain.PRStatusMerged {
		t.Fatalf("expected status MERGED after first merge, got %s", pr1.Status)
	}
	if pr1.MergedAt == nil {
		t.Fatalf("MergedAt must be set after merge")
	}

	mergedAt1 := *pr1.MergedAt

	pr2, err := svc.Merge(ctx, id)
	if err != nil {
		t.Fatalf("Merge(2) returned error: %v", err)
	}
	if pr2.Status != domain.PRStatusMerged {
		t.Fatalf("expected status MERGED after second merge, got %s", pr2.Status)
	}
	if pr2.MergedAt == nil {
		t.Fatalf("MergedAt must stay set")
	}
	if !mergedAt1.Equal(*pr2.MergedAt) {
		t.Fatalf("MergedAt must not change on idempotent merge")
	}
}

func TestPRService_Reassign_HappyPath(t *testing.T) {
	ctx := context.Background()

	usersRepo := newFakeUserRepo()
	prRepo := newFakePRRepo()

	team := domain.TeamName("team-review")

	old := domain.User{ID: "u2", Username: "OldReviewer", TeamName: team, IsActive: true}
	new1 := domain.User{ID: "u3", Username: "Candidate1", TeamName: team, IsActive: true}
	new2 := domain.User{ID: "u4", Username: "Candidate2", TeamName: team, IsActive: true}
	author := domain.User{ID: "u1", Username: "Author", TeamName: "another", IsActive: true}

	usersRepo.users[author.ID] = author
	usersRepo.users[old.ID] = old
	usersRepo.users[new1.ID] = new1
	usersRepo.users[new2.ID] = new2

	prID := domain.PullRequestID("pr-reassign")

	prRepo.prs[prID] = domain.PullRequest{
		ID:                prID,
		Name:              "PR reassign",
		AuthorID:          author.ID,
		Status:            domain.PRStatusOpen,
		AssignedReviewers: []domain.UserID{old.ID, "u5"},
	}

	svc := &PRService{
		Users: usersRepo,
		Prs:   prRepo,
		Rand:  rand.New(rand.NewSource(1)),
	}

	updated, newReviewer, err := svc.Reassign(ctx, prID, old.ID)
	if err != nil {
		t.Fatalf("Reassign returned error: %v", err)
	}

	if newReviewer == old.ID {
		t.Fatalf("new reviewer must differ from old")
	}
	if newReviewer != new1.ID && newReviewer != new2.ID {
		t.Fatalf("unexpected new reviewer: %s", newReviewer)
	}

	for _, rID := range updated.AssignedReviewers {
		if rID == old.ID {
			t.Fatalf("old reviewer must be removed from assigned list")
		}
	}
}

func TestPRService_Reassign_MergedPR(t *testing.T) {
	ctx := context.Background()

	usersRepo := newFakeUserRepo()
	prRepo := newFakePRRepo()

	prID := domain.PullRequestID("pr-merged")
	prRepo.prs[prID] = domain.PullRequest{
		ID:                prID,
		Name:              "Merged PR",
		AuthorID:          "u1",
		Status:            domain.PRStatusMerged,
		AssignedReviewers: []domain.UserID{"u2"},
	}

	svc := &PRService{
		Users: usersRepo,
		Prs:   prRepo,
		Rand:  rand.New(rand.NewSource(1)),
	}

	_, _, err := svc.Reassign(ctx, prID, "u2")
	if err == nil {
		t.Fatalf("expected error on reassigning merged PR")
	}
	if err != domain.ErrPullRequestMerged {
		t.Fatalf("expected ErrPullRequestMerged, got %v", err)
	}
}

func TestPRService_Reassign_NotAssigned(t *testing.T) {
	ctx := context.Background()

	usersRepo := newFakeUserRepo()
	prRepo := newFakePRRepo()

	team := domain.TeamName("t")
	usersRepo.users["u2"] = domain.User{ID: "u2", TeamName: team, IsActive: true}
	usersRepo.users["u3"] = domain.User{ID: "u3", TeamName: team, IsActive: true}

	prID := domain.PullRequestID("pr-not-assigned")
	prRepo.prs[prID] = domain.PullRequest{
		ID:                prID,
		Name:              "PR not assigned",
		AuthorID:          "u1",
		Status:            domain.PRStatusOpen,
		AssignedReviewers: []domain.UserID{"u3"},
	}

	svc := &PRService{
		Users: usersRepo,
		Prs:   prRepo,
		Rand:  rand.New(rand.NewSource(1)),
	}

	_, _, err := svc.Reassign(ctx, prID, "u2")
	if err == nil {
		t.Fatalf("expected error when oldUser is not assigned")
	}
	if err != domain.ErrNotAssigned {
		t.Fatalf("expected ErrNotAssigned, got %v", err)
	}
}

func TestPRService_Reassign_NoCandidate(t *testing.T) {
	ctx := context.Background()

	usersRepo := newFakeUserRepo()
	prRepo := newFakePRRepo()

	team := domain.TeamName("t")
	old := domain.User{ID: "u2", TeamName: team, IsActive: true}
	usersRepo.users[old.ID] = old

	prID := domain.PullRequestID("pr-no-candidate")
	prRepo.prs[prID] = domain.PullRequest{
		ID:                prID,
		Name:              "PR no candidate",
		AuthorID:          "u1",
		Status:            domain.PRStatusOpen,
		AssignedReviewers: []domain.UserID{old.ID},
	}

	svc := &PRService{
		Users: usersRepo,
		Prs:   prRepo,
		Rand:  rand.New(rand.NewSource(1)),
	}

	_, _, err := svc.Reassign(ctx, prID, old.ID)
	if err == nil {
		t.Fatalf("expected error when no candidates")
	}
	if err != domain.ErrNoCandidate {
		t.Fatalf("expected ErrNoCandidate, got %v", err)
	}
}
