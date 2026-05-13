package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/6ermvH/log-parser/internal/parser"
)

type parseSubmitter interface {
	Submit(ctx context.Context, path string) (uuid.UUID, error)
}

type parseRequest struct {
	Path string `json:"path"`
}

type parseResponse struct {
	LogID string `json:"log_id"`
}

// parseHandler accepts a log archive for asynchronous parsing.
//
//	@Summary		Submit a log archive for parsing
//	@Description	Accepts a path to a zip archive (relative to data/), persists a `processing` log entry and runs parsing in background. Use GET /api/v1/log/{log_id} to poll status.
//	@Tags			parse
//	@Accept			json
//	@Produce		json
//	@Param			request	body		parseRequest	true	"Path to log archive (relative to data/)"
//	@Success		202		{object}	parseResponse	"log_id; parsing is queued"
//	@Failure		400		{object}	errorResponse	"validation error (bad body, empty path, path outside data/, file not found, not a zip archive)"
//	@Failure		500		{object}	errorResponse	"internal error (failed to persist processing log)"
//	@Router			/api/v1/parse [post]
func parseHandler(svc parseSubmitter, log *slog.Logger, dataDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req parseRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, log, http.StatusBadRequest, "invalid request body")

			return
		}

		if strings.TrimSpace(req.Path) == "" {
			writeError(w, log, http.StatusBadRequest, "path is required")

			return
		}

		resolvedPath, err := resolveDataPath(dataDir, req.Path)
		if err != nil {
			writeError(w, log, http.StatusBadRequest, err.Error())

			return
		}

		logID, err := svc.Submit(r.Context(), resolvedPath)
		if err != nil {
			switch {
			case errors.Is(err, parser.ErrInputNotFound):
				writeError(w, log, http.StatusBadRequest, "input file not found")
			case errors.Is(err, parser.ErrInputNotZip):
				writeError(w, log, http.StatusBadRequest, "input is not a valid zip archive")
			default:
				log.Error("submit parse", "err", err)
				writeError(w, log, http.StatusInternalServerError, "internal server error")
			}

			return
		}

		writeJSON(w, log, http.StatusAccepted, parseResponse{LogID: logID.String()})
	}
}

func resolveDataPath(dataDir, requested string) (string, error) {
	cleanedDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}

	candidate := requested
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(cleanedDir, candidate)
	}

	cleaned, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(cleanedDir, cleaned)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errPathOutsideDataDir
	}

	return cleaned, nil
}
