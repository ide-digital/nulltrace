package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, dbPath string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	dsn := sqliteDSN(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := pingPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func sqliteDSN(dbPath string) string {
	abs := dbPath
	if a, err := filepath.Abs(dbPath); err == nil {
		abs = a
	}
	slash := filepath.ToSlash(abs)
	if runtime.GOOS == "windows" {
		if !strings.HasPrefix(slash, "/") {
			slash = "/" + slash
		}
		return "file://" + slash + "?mode=rwc"
	}
	u := url.URL{Scheme: "file", Path: slash}
	return u.String()
}

func pingPragmas(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping sqlite: %w", err)
	}
	pragmas := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`PRAGMA synchronous = NORMAL;`,
		`PRAGMA secure_delete = ON;`,
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			return fmt.Errorf("%s: %w", strings.TrimSuffix(p, ";"), err)
		}
	}
	var fk int
	if err := db.QueryRowContext(ctx, `PRAGMA foreign_keys;`).Scan(&fk); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}
	if fk != 1 {
		return fmt.Errorf("foreign_keys pragma did not stick")
	}
	return nil
}

func SplitStatements(sqlText string) []string {
	var out []string
	var b strings.Builder
	depth := 0
	for _, line := range strings.Split(sqlText, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		upper := strings.ToUpper(trim)
		if upper == "BEGIN" || strings.HasSuffix(upper, " BEGIN") {
			depth++
		}
		if strings.HasPrefix(upper, "END") && (upper == "END" || strings.HasPrefix(upper, "END;") || strings.HasPrefix(upper, "END ")) {
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(b.String())
			if stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}
