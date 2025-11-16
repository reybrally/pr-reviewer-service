package service

import (
	"context"
	"pr-reviewer-service/internal/domain"
)

type UserService struct {
	users domain.UserRepository
}

func NewUserService(users domain.UserRepository) *UserService {
	return &UserService{
		users: users,
	}
}

func (s *UserService) SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (domain.User, error) {
	user, err := s.users.SetIsActive(ctx, id, isActive)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}
