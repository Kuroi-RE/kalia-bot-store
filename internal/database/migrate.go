package database

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migration/*.sql
var migrationFS embed.FS

// Migrate applies all pending up migrations using the embedded SQL files.
func Migrate(dsn string) error {
	src, err := iofs.New(migrationFS, "migration")
	if err != nil {
		return fmt.Errorf("load migration source: %w", err)
	}

	// golang-migrate expects a "pgx5://" scheme for the pgx v5 driver.
	m, err := migrate.NewWithSourceInstance("iofs", src, normalizeDSN(dsn))
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// normalizeDSN rewrites postgres:// / postgresql:// to the pgx5:// scheme.
func normalizeDSN(dsn string) string {
	for _, prefix := range []string{"postgresql://", "postgres://"} {
		if len(dsn) >= len(prefix) && dsn[:len(prefix)] == prefix {
			return "pgx5://" + dsn[len(prefix):]
		}
	}
	return dsn
}

var _ = pgx.ErrNilConfig // ensure the pgx5 migrate driver is linked
