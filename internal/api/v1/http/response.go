package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

type healthResponse struct {
	Status string `json:"status"`
}

func writeJSON(w http.ResponseWriter, log *slog.Logger, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Error("write json response", "err", err)
	}
}

func writeError(w http.ResponseWriter, log *slog.Logger, status int, message string) {
	writeJSON(w, log, status, errorResponse{Error: message})
}
