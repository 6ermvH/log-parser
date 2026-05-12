package migrate

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const (
	schemePostgres = "postgres://"
	schemePgx5     = "pgx5://"
	sourceName     = "iofs"
)

func Run(fsys fs.FS, dsn string) error {
	src, err := iofs.New(fsys, ".")
	if err != nil {
		return fmt.Errorf("iofs source: %w", err)
	}

	migrateDSN := strings.Replace(dsn, schemePostgres, schemePgx5, 1)

	m, err := migrate.NewWithSourceInstance(sourceName, src, migrateDSN)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}

	if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", upErr)
	}

	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return fmt.Errorf("migrate close source: %w", srcErr)
	}

	if dbErr != nil {
		return fmt.Errorf("migrate close db: %w", dbErr)
	}

	return nil
}
