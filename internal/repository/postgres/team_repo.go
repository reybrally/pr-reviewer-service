package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"pr-reviewer-service/internal/domain"
)

type TeamRepo struct {
	db *sql.DB
}

func NewTeamRepo(db *sql.DB) *TeamRepo {
	return &TeamRepo{db: db}
}

func (r *TeamRepo) CreateTeam(ctx context.Context, name domain.TeamName) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO teams (team_name)
        VALUES ($1)
        ON CONFLICT (team_name) DO NOTHING
    `, string(name))
	if err != nil {
		return fmt.Errorf("create team: %w", err)
	}
	return nil
}

func (r *TeamRepo) GetTeam(ctx context.Context, name domain.TeamName) (domain.Team, error) {
	var team domain.Team
	team.Name = name

	var exists bool
	err := r.db.QueryRowContext(ctx, `
        SELECT EXISTS (SELECT 1 FROM teams WHERE team_name = $1)
    `, string(name)).Scan(&exists)
	if err != nil {
		return domain.Team{}, fmt.Errorf("get team exists: %w", err)
	}
	if !exists {
		return domain.Team{}, domain.ErrNotFound
	}

	rows, err := r.db.QueryContext(ctx, `
        SELECT user_id, username, is_active
        FROM users
        WHERE team_name = $1
        ORDER BY user_id
    `, string(name))
	if err != nil {
		return domain.Team{}, fmt.Errorf("get team members: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var members []domain.User
	for rows.Next() {
		var id, username string
		var active bool
		if err := rows.Scan(&id, &username, &active); err != nil {
			return domain.Team{}, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, domain.User{
			ID:       domain.UserID(id),
			Username: username,
			TeamName: name,
			IsActive: active,
		})
	}
	if err := rows.Err(); err != nil {
		return domain.Team{}, fmt.Errorf("iterate members: %w", err)
	}

	team.Members = members
	return team, nil
}

func (r *TeamRepo) TeamExists(ctx context.Context, name domain.TeamName) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
        SELECT EXISTS (SELECT 1 FROM teams WHERE team_name = $1)
    `, string(name)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("team exists: %w", err)
	}
	return exists, nil
}
