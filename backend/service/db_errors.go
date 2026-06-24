package service

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func isPostgresErrorCode(err error, code string) bool {
	if err == nil || code == "" {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}
