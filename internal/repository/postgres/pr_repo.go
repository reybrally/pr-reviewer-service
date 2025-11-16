package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pr-reviewer-service/internal/domain"
)

type PullRequestRepo struct {
	db *sql.DB
}

func NewPullRequestRepo(db *sql.DB) *PullRequestRepo {
	return &PullRequestRepo{db: db}
}

func (r *PullRequestRepo) Create(ctx context.Context, pr domain.PullRequest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create pr begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
        INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, created_at, merged_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `,
		string(pr.ID),
		pr.Name,
		string(pr.AuthorID),
		string(pr.Status),
		pr.CreatedAt,
		pr.MergedAt,
	)
	if err != nil {
		return fmt.Errorf("insert pull_request: %w", err)
	}

	if len(pr.AssignedReviewers) > 0 {
		stmt, err := tx.PrepareContext(ctx, `
            INSERT INTO pull_request_reviewers (pull_request_id, user_id)
            VALUES ($1, $2)
        `)
		if err != nil {
			return fmt.Errorf("prepare insert reviewers: %w", err)
		}
		defer func() {
			_ = stmt.Close()
		}()

		for _, reviewerID := range pr.AssignedReviewers {
			if _, err := stmt.ExecContext(ctx, string(pr.ID), string(reviewerID)); err != nil {
				return fmt.Errorf("insert reviewer %s: %w", reviewerID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create pr commit: %w", err)
	}

	return nil
}

func (r *PullRequestRepo) Exists(ctx context.Context, id domain.PullRequestID) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `
        SELECT EXISTS (SELECT 1 FROM pull_requests WHERE pull_request_id = $1)
    `, string(id)).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("pr exists: %w", err)
	}
	return exists, nil
}

func (r *PullRequestRepo) Get(ctx context.Context, id domain.PullRequestID) (domain.PullRequest, error) {
	var pr domain.PullRequest
	var prID, name, authorID, statusStr string
	var createdAt time.Time
	var mergedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
        SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
        FROM pull_requests
        WHERE pull_request_id = $1
    `, string(id)).Scan(&prID, &name, &authorID, &statusStr, &createdAt, &mergedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.PullRequest{}, domain.ErrNotFound
		}
		return domain.PullRequest{}, fmt.Errorf("get pr: %w", err)
	}

	pr.ID = domain.PullRequestID(prID)
	pr.Name = name
	pr.AuthorID = domain.UserID(authorID)
	pr.Status = domain.PRStatus(statusStr)
	pr.CreatedAt = createdAt
	if mergedAt.Valid {
		t := mergedAt.Time
		pr.MergedAt = &t
	}

	rows, err := r.db.QueryContext(ctx, `
        SELECT user_id
        FROM pull_request_reviewers
        WHERE pull_request_id = $1
        ORDER BY user_id
    `, string(id))
	if err != nil {
		return domain.PullRequest{}, fmt.Errorf("get pr reviewers: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var reviewers []domain.UserID
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return domain.PullRequest{}, fmt.Errorf("scan reviewer: %w", err)
		}
		reviewers = append(reviewers, domain.UserID(uid))
	}
	if err := rows.Err(); err != nil {
		return domain.PullRequest{}, fmt.Errorf("iterate reviewers: %w", err)
	}

	pr.AssignedReviewers = reviewers
	return pr, nil
}

func (r *PullRequestRepo) MarkMerged(ctx context.Context, id domain.PullRequestID, mergedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
        UPDATE pull_requests
        SET status = 'MERGED',
            merged_at = $2
        WHERE pull_request_id = $1
    `, string(id), mergedAt)
	if err != nil {
		return fmt.Errorf("mark merged: %w", err)
	}
	return nil
}

func (r *PullRequestRepo) ReplaceReviewer(ctx context.Context, prID domain.PullRequestID, oldUserID, newUserID domain.UserID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace reviewer begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
        DELETE FROM pull_request_reviewers
        WHERE pull_request_id = $1 AND user_id = $2
    `, string(prID), string(oldUserID)); err != nil {
		return fmt.Errorf("delete old reviewer: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
        INSERT INTO pull_request_reviewers (pull_request_id, user_id)
        VALUES ($1, $2)
        ON CONFLICT DO NOTHING
    `, string(prID), string(newUserID)); err != nil {
		return fmt.Errorf("insert new reviewer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("replace reviewer commit: %w", err)
	}

	return nil
}

func (r *PullRequestRepo) ListByReviewer(ctx context.Context, reviewerID domain.UserID) ([]domain.PullRequestShort, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT pr.pull_request_id,
               pr.pull_request_name,
               pr.author_id,
               pr.status
        FROM pull_requests pr
        JOIN pull_request_reviewers r ON pr.pull_request_id = r.pull_request_id
        WHERE r.user_id = $1
        ORDER BY pr.created_at DESC, pr.pull_request_id
    `, string(reviewerID))
	if err != nil {
		return nil, fmt.Errorf("list prs by reviewer: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var res []domain.PullRequestShort
	for rows.Next() {
		var id, name, authorID, statusStr string
		if err := rows.Scan(&id, &name, &authorID, &statusStr); err != nil {
			return nil, fmt.Errorf("scan pr short: %w", err)
		}
		res = append(res, domain.PullRequestShort{
			ID:       domain.PullRequestID(id),
			Name:     name,
			AuthorID: domain.UserID(authorID),
			Status:   domain.PRStatus(statusStr),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate prs by reviewer: %w", err)
	}

	return res, nil
}

func (r *PullRequestRepo) StatsAssignmentsByUser(
	ctx context.Context,
) (map[domain.UserID]int, error) {
	const q = `
        SELECT user_id, COUNT(*)
        FROM pull_request_reviewers
        GROUP BY user_id
    `

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	res := make(map[domain.UserID]int)
	for rows.Next() {
		var id string
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, err
		}
		res[domain.UserID(id)] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}
