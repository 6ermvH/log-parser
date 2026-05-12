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

// healthHandler returns a liveness probe.
//
//	@Summary		Health check
//	@Description	Returns 200 if the database is reachable, 503 otherwise.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	healthResponse
//	@Failure		503	{object}	healthResponse
//	@Router			/health [get]
func healthHandler(checker healthChecker, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()

		if err := checker.Ping(ctx); err != nil {
			log.Warn("health db ping failed", "err", err)
			writeJSON(w, log, http.StatusServiceUnavailable, healthResponse{Status: "unhealthy"})

			return
		}

		writeJSON(w, log, http.StatusOK, healthResponse{Status: "ok"})
	}
}
