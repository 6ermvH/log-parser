package reaper

import (
	"context"
	"log/slog"
	"time"
)

type repo interface {
	ReapStaleProcessing(ctx context.Context, timeout time.Duration) (int, error)
}

type Reaper struct {
	repo    repo
	tick    time.Duration
	timeout time.Duration
	log     *slog.Logger
}

func New(r repo, tick, timeout time.Duration, log *slog.Logger) *Reaper {
	return &Reaper{repo: r, tick: tick, timeout: timeout, log: log}
}

func (r *Reaper) Run(ctx context.Context) {
	r.log.Info("reaper started", "tick", r.tick, "timeout", r.timeout)

	ticker := time.NewTicker(r.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.log.Info("reaper stopped")

			return
		case <-ticker.C:
			r.cycle(ctx)
		}
	}
}

func (r *Reaper) cycle(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, r.tick)
	defer cancel()

	count, err := r.repo.ReapStaleProcessing(ctx, r.timeout)
	if err != nil {
		r.log.Error("reap cycle failed", "err", err)

		return
	}

	if count > 0 {
		r.log.Info("reaped stale processing logs", "count", count)
	}
}
