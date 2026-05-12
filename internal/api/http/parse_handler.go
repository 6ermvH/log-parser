package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/6ermvH/log-parser/internal/service"
)

type parseRunner interface {
	Run(ctx context.Context, path string) (service.ParseResult, error)
}

type parseRequest struct {
	Path string `json:"path"`
}

type parseResponse struct {
	LogID string `json:"log_id"`
	Error string `json:"error,omitempty"`
}

func parseHandler(svc parseRunner, log *slog.Logger, dataDir string) http.HandlerFunc {
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

		result, err := svc.Run(r.Context(), resolvedPath)
		if err != nil {
			log.Error("parse service", "err", err)
			writeError(w, log, http.StatusInternalServerError, "internal server error")

			return
		}

		if result.ParseErr != nil {
			writeJSON(w, log, http.StatusBadRequest, parseResponse{
				LogID: result.LogID.String(),
				Error: result.ParseErr.Error(),
			})

			return
		}

		writeJSON(w, log, http.StatusCreated, parseResponse{LogID: result.LogID.String()})
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
