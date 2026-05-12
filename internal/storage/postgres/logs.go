package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type LogMeta struct {
	ID           uuid.UUID
	Status       string
	UploadedAt   time.Time
	ErrorMessage string
}

type Counts struct {
	Nodes int
	Ports int
}

const statusProcessing = "processing"

func (r *Repository) InsertProcessingLog(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO logs (id, status) VALUES ($1, $2)`,
		id, statusProcessing,
	)
	if err != nil {
		return fmt.Errorf("insert log: %w", err)
	}

	return nil
}

const statusFailed = "failed"

func (r *Repository) MarkLogFailed(ctx context.Context, id uuid.UUID, errMsg string) (err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback(ctx) //nolint:errcheck // rollback error is not actionable; original err is reported
		}
	}()

	if _, err = tx.Exec(ctx, `UPDATE logs SET status = $1 WHERE id = $2`, statusFailed, id); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	if _, err = tx.Exec(ctx,
		`INSERT INTO failed_logs (log_id, error_message) VALUES ($1, $2)`,
		id, errMsg,
	); err != nil {
		return fmt.Errorf("insert failed_log: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

const reaperErrorMessage = "stale: timed out by ETL"

func (r *Repository) ReapStaleProcessing(ctx context.Context, timeout time.Duration) (int, error) {
	const q = `
		WITH stale AS (
			UPDATE logs SET status = 'failed'
			WHERE status = 'processing' AND uploaded_at < now() - $1::interval
			RETURNING id
		)
		INSERT INTO failed_logs (log_id, error_message)
		SELECT id, $2 FROM stale
		ON CONFLICT (log_id) DO NOTHING
		RETURNING log_id
	`

	interval := fmt.Sprintf("%f seconds", timeout.Seconds())

	rows, err := r.pool.Query(ctx, q, interval, reaperErrorMessage)
	if err != nil {
		return 0, fmt.Errorf("reap: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}

	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("reap iterate: %w", err)
	}

	return count, nil
}

func (r *Repository) GetLog(ctx context.Context, id uuid.UUID) (LogMeta, error) {
	const q = `
		SELECT l.id, l.status, l.uploaded_at, COALESCE(f.error_message, '')
		FROM logs l
		LEFT JOIN failed_logs f ON f.log_id = l.id
		WHERE l.id = $1
	`

	var m LogMeta
	if err := r.pool.QueryRow(ctx, q, id).Scan(&m.ID, &m.Status, &m.UploadedAt, &m.ErrorMessage); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LogMeta{}, ErrNotFound
		}

		return LogMeta{}, fmt.Errorf("get log: %w", err)
	}

	return m, nil
}

func (r *Repository) CountByLog(ctx context.Context, logID uuid.UUID) (Counts, error) {
	const q = `
		SELECT
			(SELECT COUNT(*) FROM nodes WHERE log_id = $1),
			(SELECT COUNT(*) FROM ports p JOIN nodes n ON p.node_id = n.id WHERE n.log_id = $1)
	`

	var c Counts
	if err := r.pool.QueryRow(ctx, q, logID).Scan(&c.Nodes, &c.Ports); err != nil {
		return Counts{}, fmt.Errorf("count: %w", err)
	}

	return c, nil
}
