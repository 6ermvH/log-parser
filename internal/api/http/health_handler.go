package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const healthPingTimeout = 2 * time.Second

type healthChecker interface {
	Ping(ctx context.Context) error
}

func healthHandler(checker healthChecker, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()

		if err := checker.Ping(ctx); err != nil {
			log.Warn("health db ping failed", "err", err)
			writeJSON(w, log, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})

			return
		}

		writeJSON(w, log, http.StatusOK, map[string]string{"status": "ok"})
	}
}
