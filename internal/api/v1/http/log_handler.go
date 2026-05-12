package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/6ermvH/log-parser/internal/service"
)

type logMetaQuery interface {
	GetLogMeta(ctx context.Context, id uuid.UUID) (service.LogMeta, error)
}

type logResponse struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	UploadedAt time.Time `json:"uploaded_at"`
	NodesCount int       `json:"nodes_count"`
	PortsCount int       `json:"ports_count"`
	Error      string    `json:"error,omitempty"`
}

// logMetaHandler returns log meta information.
//
//	@Summary		Get log meta
//	@Description	Returns log status, upload time, nodes/ports counts and optional error.
//	@Tags			log
//	@Produce		json
//	@Param			log_id	path		string	true	"Log UUID"
//	@Success		200		{object}	logResponse
//	@Failure		400		{object}	errorResponse
//	@Failure		404		{object}	errorResponse
//	@Failure		500		{object}	errorResponse
//	@Router			/api/v1/log/{log_id} [get]
func logMetaHandler(q logMetaQuery, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("log_id"))
		if err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid log_id")

			return
		}

		meta, err := q.GetLogMeta(r.Context(), id)
		if err != nil {
			if errors.Is(err, service.ErrNotFound) {
				writeError(w, log, http.StatusNotFound, "log not found")

				return
			}

			log.Error("get log meta", "err", err, "log_id", id)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		writeJSON(w, log, http.StatusOK, logResponse{
			ID:         meta.ID.String(),
			Status:     meta.Status,
			UploadedAt: meta.UploadedAt,
			NodesCount: meta.NodesCount,
			PortsCount: meta.PortsCount,
			Error:      meta.ErrorMessage,
		})
	}
}
