package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"
)

//go:embed schema.sql
var schemaFS embed.FS

const currentSchemaVersion = 1

// Migrate applies embedded DDL if the vault is at a lower schema version.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	raw, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	var version int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version >= currentSchemaVersion {
		return tx.Commit()
	}

	for _, stmt := range SplitStatements(string(raw)) {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("apply schema statement %q: %w", truncate(stmt, 80), err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
		currentSchemaVersion, now,
	); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	s = stringMinWhitespace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func stringMinWhitespace(s string) string {
	out := make([]byte, 0, len(s))
	prevSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			if prevSpace {
				continue
			}
			prevSpace = true
			out = append(out, ' ')
			continue
		}
		prevSpace = false
		out = append(out, c)
	}
	return string(out)
}
