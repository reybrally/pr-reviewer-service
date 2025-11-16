package service

import (
	"math/rand"
	"pr-reviewer-service/internal/domain"
)

func pickRandomReviewers(users []domain.User, limit int, rnd *rand.Rand) []domain.UserID {
	if len(users) == 0 || limit <= 0 {
		return nil
	}

	indices := rnd.Perm(len(users))
	n := limit
	if len(users) < limit {
		n = len(users)
	}

	res := make([]domain.UserID, 0, n)
	for i := 0; i < n; i++ {
		u := users[indices[i]]
		res = append(res, u.ID)
	}

	return res
}
