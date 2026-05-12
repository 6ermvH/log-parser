package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/6ermvH/log-parser/internal/domain"
)

type parseStorage interface {
	InsertProcessingLog(ctx context.Context, id uuid.UUID) error
	SaveDomainLog(ctx context.Context, id uuid.UUID, dlog domain.Log) error
	MarkLogFailed(ctx context.Context, id uuid.UUID, message string) error
}

type logParser interface {
	Parse(path string) (domain.Log, error)
}

type parseRequest struct {
	Path string `json:"path"`
}

type parseResponse struct {
	LogID string `json:"log_id"`
	Error string `json:"error,omitempty"`
}

func parseHandler(parser logParser, storage parseStorage, log *slog.Logger, dataDir string) http.HandlerFunc {
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

		logID, err := uuid.NewV7()
		if err != nil {
			log.Error("generate uuid", "err", err)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		if err := storage.InsertProcessingLog(r.Context(), logID); err != nil {
			log.Error("insert processing log", "err", err, "log_id", logID)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		dlog, parseErr := parser.Parse(resolvedPath)
		if parseErr != nil {
			log.Warn("parse failed", "err", parseErr, "log_id", logID, "path", resolvedPath)

			if markErr := storage.MarkLogFailed(r.Context(), logID, parseErr.Error()); markErr != nil {
				log.Error("mark log failed", "err", markErr, "log_id", logID)
			}

			writeJSON(w, log, http.StatusBadRequest, parseResponse{LogID: logID.String(), Error: parseErr.Error()})

			return
		}

		if err := storage.SaveDomainLog(r.Context(), logID, dlog); err != nil {
			log.Error("save domain log", "err", err, "log_id", logID)

			if markErr := storage.MarkLogFailed(r.Context(), logID, "save failed: "+err.Error()); markErr != nil {
				log.Error("mark log failed after save error", "err", markErr, "log_id", logID)
			}

			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		writeJSON(w, log, http.StatusCreated, parseResponse{LogID: logID.String()})
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
