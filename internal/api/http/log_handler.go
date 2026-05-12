package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	pg "github.com/6ermvH/log-parser/internal/storage/postgres"
)

type logStorage interface {
	GetLog(ctx context.Context, id uuid.UUID) (pg.LogMeta, error)
	CountByLog(ctx context.Context, logID uuid.UUID) (pg.Counts, error)
}

type logResponse struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	UploadedAt time.Time `json:"uploaded_at"`
	NodesCount int       `json:"nodes_count"`
	PortsCount int       `json:"ports_count"`
	Error      string    `json:"error,omitempty"`
}

func logMetaHandler(storage logStorage, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("log_id"))
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid log_id")

			return
		}

		meta, err := storage.GetLog(r.Context(), id)
		if err != nil {
			if errors.Is(err, pg.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "log not found")

				return
			}

			log.Error("get log", "err", err, "log_id", id)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		counts, err := storage.CountByLog(r.Context(), id)
		if err != nil {
			log.Error("count by log", "err", err, "log_id", id)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		writeJSON(w, log, http.StatusOK, logResponse{
			ID:         meta.ID.String(),
			Status:     meta.Status,
			UploadedAt: meta.UploadedAt,
			NodesCount: counts.Nodes,
			PortsCount: counts.Ports,
			Error:      meta.ErrorMessage,
		})
	}
}
