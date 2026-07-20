package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("record not found")

// ErrNoStock is returned when no AVAILABLE account exists to reserve.
var ErrNoStock = errors.New("out of stock")

// IsNotFound reports whether err is a no-rows error.
func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrNotFound)
}

// IsForeignKeyViolation reports whether err is a Postgres FK-violation error
// (SQLSTATE 23503) — e.g. deleting a row still referenced by another table.
func IsForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// IsUniqueViolation reports whether err is a Postgres unique-constraint error
// (SQLSTATE 23505), optionally matching a specific constraint name.
func IsUniqueViolation(err error, constraint ...string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	if len(constraint) == 0 {
		return true
	}
	for _, c := range constraint {
		if pgErr.ConstraintName == c {
			return true
		}
	}
	return false
}
