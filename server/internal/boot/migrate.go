// Package boot — embedded migration runner.
//
// Reads SQL files from manifest/migrations/, tracks applied migrations
// in a `schema_migrations` table, runs pending ones on startup.
// No external deps needed — uses g.DB().
package boot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

const migrationsDir = "migrations"

// RunMigrations scans the migrations directory and applies any pending SQL files.
func RunMigrations(ctx context.Context) error {
	// Find migration files.
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return gerror.Wrapf(err, "read migrations dir %s", migrationsDir)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		g.Log().Info(ctx, "[migrate] no migration files found")
		return nil
	}

	// Ensure tracking table.
	if err := ensureMigrationsTable(ctx); err != nil {
		return err
	}

	// Load already-applied.
	applied := make(map[string]bool)
	rows, err := g.DB().Model("schema_migrations").Ctx(ctx).
		Fields("version").Where("applied", true).All()
	if err != nil {
		return gerror.Wrap(err, "query applied migrations")
	}
	for _, row := range rows {
		applied[row["version"].String()] = true
	}

	g.Log().Infof(ctx, "[migrate] %d/%d already applied, checking pending...",
		len(applied), len(files))

	// Apply pending.
	for _, f := range files {
		ver := strings.TrimSuffix(f, ".sql")
		if applied[ver] {
			continue
		}

		upSQL, err := extractUpMigration(filepath.Join(migrationsDir, f))
		if err != nil {
			return gerror.Wrapf(err, "parse migration %s", f)
		}
		if upSQL == "" {
			continue
		}

		g.Log().Infof(ctx, "[migrate] applying %s...", f)

		// Run in transaction.
		if err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			for _, stmt := range splitStatements(upSQL) {
				if _, err := tx.Exec(stmt); err != nil {
					return gerror.Wrapf(err, "migration %s failed:\n%s", f, stmt)
				}
			}
			_, err := tx.Model("schema_migrations").Ctx(ctx).Data(g.Map{
				"version": ver,
				"applied": true,
			}).Insert()
			return err
		}); err != nil {
			return err
		}
		g.Log().Infof(ctx, "[migrate] %s applied", f)
	}

	return nil
}

func ensureMigrationsTable(ctx context.Context) error {
	_, err := g.DB().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    VARCHAR(255) PRIMARY KEY,
			applied    BOOLEAN NOT NULL DEFAULT false,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	return gerror.Wrap(err, "create schema_migrations table")
}

// extractUpMigration returns the SQL between `-- +goose Up` and `-- +goose Down`
// (or end of file), stripping goose directives.
func extractUpMigration(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(raw)

	// Find +goose Up marker.
	upIdx := strings.Index(text, "-- +goose Up")
	if upIdx < 0 {
		// No marker — use entire file.
		return stripGooseDirectives(text), nil
	}
	body := text[upIdx+len("-- +goose Up"):]

	// Stop at +goose Down marker.
	downIdx := strings.Index(body, "-- +goose Down")
	if downIdx >= 0 {
		body = body[:downIdx]
	}

	return stripGooseDirectives(body), nil
}

// stripGooseDirectives removes goose-specific annotation lines.
func stripGooseDirectives(sql string) string {
	lines := strings.Split(sql, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-- +goose") {
			continue // skip goose directives
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// splitStatements splits on semicolons, skipping goose directives.
func splitStatements(sql string) []string {
	var stmts []string
	for _, part := range strings.Split(sql, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Skip lines that are only goose directives.
		lines := strings.Split(part, "\n")
		hasSQL := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "--") {
				continue
			}
			hasSQL = true
			break
		}
		if !hasSQL {
			continue
		}
		stmts = append(stmts, part+";")
	}
	return stmts
}

func init() {
	// Migration filename format: NNNN_name.sql
	// Validator: first segment must be digits.
	for _, f := range listMigrationFiles() {
		ver := strings.TrimSuffix(f, ".sql")
		parts := strings.SplitN(ver, "_", 2)
		if len(parts) < 1 || parts[0] == "" {
			panic(fmt.Sprintf("[migrate] invalid migration filename: %s (must be NNNN_name.sql)", f))
		}
		for _, c := range parts[0] {
			if c < '0' || c > '9' {
				panic(fmt.Sprintf("[migrate] invalid migration filename: %s (must start with digits)", f))
			}
		}
	}
}

func listMigrationFiles() []string {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}
