package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"pr-reviewer-service/internal/domain"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) UpsertUsers(ctx context.Context, users []domain.User) error {
	if len(users) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert users begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO users (user_id, username, team_name, is_active)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (user_id) DO UPDATE
        SET username = EXCLUDED.username,
            team_name = EXCLUDED.team_name,
            is_active = EXCLUDED.is_active
    `)
	if err != nil {
		return fmt.Errorf("prepare upsert users: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, u := range users {
		if _, err := stmt.ExecContext(ctx,
			string(u.ID),
			u.Username,
			string(u.TeamName),
			u.IsActive,
		); err != nil {
			return fmt.Errorf("exec upsert user %s: %w", u.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert users commit: %w", err)
	}

	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	var userID, username, teamName string
	var isActive bool

	err := r.db.QueryRowContext(ctx, `
        SELECT user_id, username, team_name, is_active
        FROM users
        WHERE user_id = $1
    `, string(id)).Scan(&userID, &username, &teamName, &isActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.User{}, domain.ErrNotFound
		}
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}

	return domain.User{
		ID:       domain.UserID(userID),
		Username: username,
		TeamName: domain.TeamName(teamName),
		IsActive: isActive,
	}, nil
}

func (r *UserRepo) SetIsActive(ctx context.Context, id domain.UserID, isActive bool) (domain.User, error) {
	_, err := r.db.ExecContext(ctx, `
        UPDATE users
        SET is_active = $2
        WHERE user_id = $1
    `, string(id), isActive)
	if err != nil {
		return domain.User{}, fmt.Errorf("set is_active: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *UserRepo) ListActiveByTeam(ctx context.Context, teamName domain.TeamName) ([]domain.User, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT user_id, username, is_active
        FROM users
        WHERE team_name = $1
          AND is_active = TRUE
        ORDER BY user_id
    `, string(teamName))
	if err != nil {
		return nil, fmt.Errorf("list active users by team: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var res []domain.User
	for rows.Next() {
		var id, username string
		var active bool
		if err := rows.Scan(&id, &username, &active); err != nil {
			return nil, fmt.Errorf("scan active user: %w", err)
		}
		res = append(res, domain.User{
			ID:       domain.UserID(id),
			Username: username,
			TeamName: teamName,
			IsActive: active,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active users: %w", err)
	}

	return res, nil
}
