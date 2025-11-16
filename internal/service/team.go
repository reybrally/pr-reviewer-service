package service

import (
	"context"
	"pr-reviewer-service/internal/domain"
)

type TeamService struct {
	teams domain.TeamRepository
	users domain.UserRepository
}

func NewTeamService(teams domain.TeamRepository, users domain.UserRepository) *TeamService {
	return &TeamService{
		teams: teams,
		users: users,
	}
}

func (s *TeamService) AddTeam(ctx context.Context, name domain.TeamName, members []domain.User) (domain.Team, error) {
	exists, err := s.teams.TeamExists(ctx, name)
	if err != nil {
		return domain.Team{}, err
	}
	if exists {
		return domain.Team{}, domain.ErrTeamExists
	}

	if err := s.teams.CreateTeam(ctx, name); err != nil {
		return domain.Team{}, err
	}

	for i := range members {
		members[i].TeamName = name
	}

	if err := s.users.UpsertUsers(ctx, members); err != nil {
		return domain.Team{}, err
	}

	team, err := s.teams.GetTeam(ctx, name)
	if err != nil {
		return domain.Team{}, err
	}

	return team, nil
}

func (s *TeamService) GetTeam(ctx context.Context, name domain.TeamName) (domain.Team, error) {
	team, err := s.teams.GetTeam(ctx, name)
	if err != nil {
		return domain.Team{}, err
	}
	return team, nil
}
