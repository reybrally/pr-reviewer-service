CREATE TABLE IF NOT EXISTS teams (
                                     team_name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS users (
                                     user_id   TEXT PRIMARY KEY,
                                     username  TEXT NOT NULL,
                                     team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
    );

CREATE TABLE IF NOT EXISTS pull_requests (
                                             pull_request_id   TEXT PRIMARY KEY,
                                             pull_request_name TEXT NOT NULL,
                                             author_id         TEXT NOT NULL REFERENCES users(user_id),
    status            TEXT NOT NULL CHECK (status IN ('OPEN', 'MERGED')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_at         TIMESTAMPTZ
    );

CREATE TABLE IF NOT EXISTS pull_request_reviewers (
                                                      pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(user_id),
    PRIMARY KEY (pull_request_id, user_id)
    );

CREATE INDEX IF NOT EXISTS idx_users_team_name ON users(team_name);
CREATE INDEX IF NOT EXISTS idx_pr_reviewers_user ON pull_request_reviewers(user_id);