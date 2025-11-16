package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed *.sql
var files embed.FS

func Run(ctx context.Context, db *sql.DB) error {
	script, err := files.ReadFile("001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	parts := strings.Split(string(script), ";")
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration statement %q: %w", stmt, err)
		}
	}

	return nil
}
