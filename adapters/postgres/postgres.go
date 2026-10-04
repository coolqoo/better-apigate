// Package postgres implements the application's durable storage boundaries.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB owns the application's shared connection pool. Rebind only translates
// positional placeholders; queries and schema use PostgreSQL semantics.
type DB struct{ *sql.DB }
type Tx struct{ *sql.Tx }

func Open(dsn string) (*DB, error) {
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("APIGATE_DATABASE_DSN must be a PostgreSQL connection URL")
	}
	pool, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	maxConns := 32
	if value := os.Getenv("APIGATE_DB_MAX_CONNS"); value != "" {
		var err error
		maxConns, err = strconv.Atoi(value)
		if err != nil || maxConns < 2 || maxConns > 500 {
			pool.Close()
			return nil, fmt.Errorf("APIGATE_DB_MAX_CONNS must be between 2 and 500")
		}
	}
	pool.SetMaxOpenConns(maxConns)
	pool.SetMaxIdleConns(min(maxConns, 8))
	pool.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = pool.PingContext(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{DB: pool}, nil
}

// Rebind accepts portable positional SQL and preserves quoted literals.
func Rebind(query string) string {
	var b strings.Builder
	n := 0
	var quote byte
	for i := 0; i < len(query); i++ {
		c := query[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				if i+1 < len(query) && query[i+1] == quote {
					i++
					b.WriteByte(query[i])
				} else {
					quote = 0
				}
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// SchemaSQL maps the declarative module schema's portable column types.
func SchemaSQL(s string) string {
	return strings.NewReplacer(" DATETIME", " TIMESTAMPTZ", " BLOB", " BYTEA", " REAL", " DOUBLE PRECISION", " INTEGER", " BIGINT").Replace(s)
}

func (d *DB) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	return d.DB.ExecContext(c, Rebind(q), a...)
}
func (d *DB) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	return d.DB.QueryContext(c, Rebind(q), a...)
}
func (d *DB) QueryRowContext(c context.Context, q string, a ...any) *sql.Row {
	return d.DB.QueryRowContext(c, Rebind(q), a...)
}
func (d *DB) Exec(q string, a ...any) (sql.Result, error) {
	return d.ExecContext(context.Background(), q, a...)
}
func (d *DB) Query(q string, a ...any) (*sql.Rows, error) {
	return d.QueryContext(context.Background(), q, a...)
}
func (d *DB) QueryRow(q string, a ...any) *sql.Row {
	return d.QueryRowContext(context.Background(), q, a...)
}
func (d *DB) BeginTx(c context.Context, o *sql.TxOptions) (*Tx, error) {
	t, e := d.DB.BeginTx(c, o)
	if e != nil {
		return nil, e
	}
	return &Tx{t}, nil
}
func (d *DB) Begin() (*Tx, error) { return d.BeginTx(context.Background(), nil) }
func (t *Tx) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	return t.Tx.ExecContext(c, Rebind(q), a...)
}
func (t *Tx) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(c, Rebind(q), a...)
}
func (t *Tx) QueryRowContext(c context.Context, q string, a ...any) *sql.Row {
	return t.Tx.QueryRowContext(c, Rebind(q), a...)
}
func (t *Tx) Exec(q string, a ...any) (sql.Result, error) {
	return t.ExecContext(context.Background(), q, a...)
}
func (t *Tx) PrepareContext(c context.Context, q string) (*sql.Stmt, error) {
	return t.Tx.PrepareContext(c, Rebind(q))
}

func (d *DB) Migrate() error {
	ctx := context.Background()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(734018921)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP)"); err != nil {
		return err
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var exists bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=?)", entry.Name()).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		content, e := migrations.ReadFile("migrations/" + entry.Name())
		if e != nil {
			return e
		}
		if _, err = tx.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES(?)", entry.Name()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
