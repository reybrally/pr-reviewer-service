package service

import (
	"context"
	"pr-reviewer-service/internal/domain"
	"testing"
)

type fakeTeamRepo struct {
	teams map[domain.TeamName]domain.Team
}

func newFakeTeamRepo() *fakeTeamRepo {
	return &fakeTeamRepo{
		teams: make(map[domain.TeamName]domain.Team),
	}
}

func (r *fakeTeamRepo) CreateTeam(ctx context.Context, name domain.TeamName) error {
	if r.teams == nil {
		r.teams = make(map[domain.TeamName]domain.Team)
	}
	r.teams[name] = domain.Team{}
	return nil
}

func (r *fakeTeamRepo) GetTeam(ctx context.Context, name domain.TeamName) (domain.Team, error) {
	t, ok := r.teams[name]
	if !ok {
		return domain.Team{}, domain.ErrNotFound
	}
	return t, nil
}

func (r *fakeTeamRepo) TeamExists(ctx context.Context, name domain.TeamName) (bool, error) {
	_, ok := r.teams[name]
	return ok, nil
}

type fakeUserRepoForTeam struct {
	upserted []domain.User
}

func newFakeUserRepoForTeam() *fakeUserRepoForTeam {
	return &fakeUserRepoForTeam{
		upserted: make([]domain.User, 0),
	}
}

func (r *fakeUserRepoForTeam) UpsertUsers(ctx context.Context, users []domain.User) error {
	r.upserted = append(r.upserted, users...)
	return nil
}

func (r *fakeUserRepoForTeam) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (r *fakeUserRepoForTeam) SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

func (r *fakeUserRepoForTeam) ListActiveByTeam(ctx context.Context, teamName domain.TeamName) ([]domain.User, error) {
	return nil, nil
}

func TestTeamService_AddTeam_Success(t *testing.T) {
	ctx := context.Background()

	teamRepo := newFakeTeamRepo()
	userRepo := newFakeUserRepoForTeam()

	svc := NewTeamService(teamRepo, userRepo)

	teamName := domain.TeamName("backend")
	members := []domain.User{
		{ID: "u1", Username: "Alice", IsActive: true},
		{ID: "u2", Username: "Bob", IsActive: false},
	}

	_, err := svc.AddTeam(ctx, teamName, members)
	if err != nil {
		t.Fatalf("AddTeam returned error: %v", err)
	}

	if len(userRepo.upserted) != len(members) {
		t.Fatalf("expected %d upserted users, got %d", len(members), len(userRepo.upserted))
	}

	upsertedByID := make(map[domain.UserID]domain.User)
	for _, u := range userRepo.upserted {
		upsertedByID[u.ID] = u
	}

	for _, orig := range members {
		u, ok := upsertedByID[orig.ID]
		if !ok {
			t.Fatalf("user %s was not upserted", orig.ID)
		}
		if u.TeamName != teamName {
			t.Fatalf("user %s must have TeamName=%s, got %s", orig.ID, teamName, u.TeamName)
		}
	}
}

func TestTeamService_AddTeam_AlreadyExists(t *testing.T) {
	ctx := context.Background()

	teamRepo := newFakeTeamRepo()
	userRepo := newFakeUserRepoForTeam()

	teamName := domain.TeamName("backend")
	teamRepo.teams[teamName] = domain.Team{}

	svc := NewTeamService(teamRepo, userRepo)

	_, err := svc.AddTeam(ctx, teamName, nil)
	if err == nil {
		t.Fatalf("expected ErrTeamExists, got nil")
	}
	if err != domain.ErrTeamExists {
		t.Fatalf("expected ErrTeamExists, got %v", err)
	}
}

func TestTeamService_GetTeam(t *testing.T) {
	ctx := context.Background()

	teamRepo := newFakeTeamRepo()
	userRepo := newFakeUserRepoForTeam()

	teamName := domain.TeamName("backend")
	teamRepo.teams[teamName] = domain.Team{}

	svc := NewTeamService(teamRepo, userRepo)

	_, err := svc.GetTeam(ctx, teamName)
	if err != nil {
		t.Fatalf("GetTeam returned error: %v", err)
	}

	_, err = svc.GetTeam(ctx, domain.TeamName("unknown"))
	if err == nil {
		t.Fatalf("expected ErrNotFound for unknown team")
	}
	if err != domain.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
